package db

import (
	"context"
	"fmt"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/foundation/v2/errorz"
	"github.com/openziti/foundation/v2/stringz"
	"github.com/openziti/storage/ast"
	"github.com/openziti/storage/boltz"
	"github.com/openziti/ziti/common/pb/edge_ctrl_pb"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"strings"
)

type ServicePolicyEventsKeyType string

const (
	ServicePolicyEventsKey = ServicePolicyEventsKeyType("servicePolicyEvents")
)

type serviceEventHandler struct {
	events []*ServiceEvent
}

func (self *serviceEventHandler) addServiceUpdatedEvent(stores *stores, tx *bbolt.Tx, serviceId []byte) {
	cursor := stores.edgeService.bindIdentitiesCollection.IterateLinks(tx, serviceId, true)

	for cursor != nil && cursor.IsValid() {
		self.addServiceEvent(tx, cursor.Current(), serviceId, ServiceUpdated)
		cursor.Next()
	}

	cursor = stores.edgeService.dialIdentitiesCollection.IterateLinks(tx, serviceId, true)
	for cursor != nil && cursor.IsValid() {
		self.addServiceEvent(tx, cursor.Current(), serviceId, ServiceUpdated)
		cursor.Next()
	}
}

func (self *serviceEventHandler) addServiceEvent(tx *bbolt.Tx, identityId, serviceId []byte, eventType ServiceEventType) {
	if len(self.events) == 0 {
		tx.OnCommit(func() {
			ServiceEvents.dispatchEventsAsync(self.events)
		})
	}

	self.events = append(self.events, &ServiceEvent{
		Type:       eventType,
		IdentityId: string(identityId),
		ServiceId:  string(serviceId),
	})
}

type roleAttributeChangeContext struct {
	serviceEventHandler
	mutateCtx             boltz.MutateContext
	rolesSymbol           boltz.EntitySetSymbol
	linkCollection        boltz.LinkCollection
	relatedLinkCollection boltz.LinkCollection
	denormLinkCollection  boltz.RefCountedLinkCollection
	changeHandler         func(fromId []byte, toId []byte, add bool)
	denormChangeHandler   func(fromId, toId []byte, add bool)
	servicePolicyEvents   []*edge_ctrl_pb.DataState_ServicePolicyChange
	errorz.ErrorHolder
}

func (self *roleAttributeChangeContext) tx() *bbolt.Tx {
	return self.mutateCtx.Tx()
}

func (self *roleAttributeChangeContext) addServicePolicyEvent(identityId, serviceId []byte, policyType PolicyType, add bool) {
	var eventType ServiceEventType
	if add {
		if policyType == PolicyTypeDial {
			eventType = ServiceDialAccessGained
		}
		if policyType == PolicyTypeBind {
			eventType = ServiceBindAccessGained
		}
	} else {
		if policyType == PolicyTypeDial {
			eventType = ServiceDialAccessLost
		}
		if policyType == PolicyTypeBind {
			eventType = ServiceBindAccessLost
		}
	}

	self.addServiceEvent(self.tx(), identityId, serviceId, eventType)
}

func (self *roleAttributeChangeContext) notifyOfPolicyChangeEvent(
	policyId []byte,
	relatedId []byte,
	relatedType edge_ctrl_pb.ServicePolicyRelatedEntityType,
	isAdd bool) {

	self.servicePolicyEvents = append(self.servicePolicyEvents, &edge_ctrl_pb.DataState_ServicePolicyChange{
		PolicyId:          string(policyId),
		RelatedEntityIds:  []string{string(relatedId)},
		RelatedEntityType: relatedType,
		Add:               isAdd,
	})
}

func (self *roleAttributeChangeContext) processServicePolicyEvents() {
	if len(self.servicePolicyEvents) > 0 {
		self.mutateCtx.UpdateContext(func(stateCtx context.Context) context.Context {
			eventSlice := self.servicePolicyEvents
			currentValue := stateCtx.Value(ServicePolicyEventsKey)
			if currentValue != nil {
				currentSlice := currentValue.([]*edge_ctrl_pb.DataState_ServicePolicyChange)
				eventSlice = append(eventSlice, currentSlice...)
			}
			return context.WithValue(stateCtx, ServicePolicyEventsKey, eventSlice)
		})
		self.servicePolicyEvents = nil
	}
}

func (store *baseStore[E]) validateRoleAttributes(attributes []string, holder errorz.ErrorHolder) {
	for _, attr := range attributes {
		if strings.HasPrefix(attr, "#") {
			holder.SetError(errorz.NewFieldError("role attributes may not be prefixed with #", "roleAttributes", attr))
			return
		}
		if strings.HasPrefix(attr, "@") {
			holder.SetError(errorz.NewFieldError("role attributes may not be prefixed with @", "roleAttributes", attr))
			return
		}
	}
}

func (store *baseStore[E]) updateServicePolicyRelatedRoles(ctx *roleAttributeChangeContext, entityId []byte, newRoleAttributes []boltz.FieldTypeAndValue) {
	cursor := ctx.rolesSymbol.GetStore().IterateIds(ctx.tx(), ast.BoolNodeTrue)

	entityRoles := FieldValuesToIds(newRoleAttributes)

	servicePolicyStore := store.stores.servicePolicy
	semanticSymbol := servicePolicyStore.symbolSemantic
	policyTypeSymbol := servicePolicyStore.symbolPolicyType

	isServices := ctx.rolesSymbol == servicePolicyStore.symbolServiceRoles
	isIdentity := ctx.rolesSymbol == servicePolicyStore.symbolIdentityRoles

	relatedEntityType := edge_ctrl_pb.ServicePolicyRelatedEntityType_RelatedPostureCheck
	if isServices {
		relatedEntityType = edge_ctrl_pb.ServicePolicyRelatedEntityType_RelatedService
	} else if isIdentity {
		relatedEntityType = edge_ctrl_pb.ServicePolicyRelatedEntityType_RelatedIdentity
	}

	ctx.changeHandler = func(policyId []byte, relatedId []byte, add bool) {
		ctx.notifyOfPolicyChangeEvent(policyId, relatedId, relatedEntityType, add)
	}

	for ; cursor.IsValid(); cursor.Next() {
		policyId := cursor.Current()
		roleSet := ctx.rolesSymbol.EvalStringList(ctx.tx(), policyId)
		roles, ids, err := splitRolesAndIds(roleSet)
		if err != nil {
			ctx.SetError(err)
			return
		}

		semantic := SemanticAllOf
		if _, semanticValue := semanticSymbol.Eval(ctx.tx(), policyId); semanticValue != nil {
			semantic = string(semanticValue)
		}
		policyType := PolicyTypeDial
		if fieldType, policyTypeValue := policyTypeSymbol.Eval(ctx.tx(), policyId); fieldType == boltz.TypeInt32 {
			policyType = GetPolicyTypeForId(*boltz.BytesToInt32(policyTypeValue))
		}
		if policyType == PolicyTypeDial {
			if isServices {
				ctx.denormLinkCollection = store.stores.edgeService.dialIdentitiesCollection
				ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
					ctx.addServicePolicyEvent(toId, fromId, PolicyTypeDial, add)
				}
			} else if isIdentity {
				ctx.denormLinkCollection = store.stores.identity.dialServicesCollection
				ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
					ctx.addServicePolicyEvent(fromId, toId, PolicyTypeDial, add)
				}
			} else {
				ctx.denormLinkCollection = store.stores.postureCheck.dialServicesCollection
				ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
					pfxlog.Logger().Warnf("posture check %v -> service %v - included? %v", string(fromId), string(toId), add)
					ctx.addServiceUpdatedEvent(store.stores, ctx.tx(), toId)
				}
			}
		} else if isServices {
			ctx.denormLinkCollection = store.stores.edgeService.bindIdentitiesCollection
			ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
				ctx.addServicePolicyEvent(toId, fromId, PolicyTypeBind, add)
			}
		} else if isIdentity {
			ctx.denormLinkCollection = store.stores.identity.bindServicesCollection
			ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
				ctx.addServicePolicyEvent(fromId, toId, PolicyTypeBind, add)
			}
		} else {
			ctx.denormLinkCollection = store.stores.postureCheck.bindServicesCollection
			ctx.denormChangeHandler = func(fromId, toId []byte, add bool) {
				pfxlog.Logger().Warnf("posture check %v -> service %v - included? %v", string(fromId), string(toId), add)
				ctx.addServiceUpdatedEvent(store.stores, ctx.tx(), toId)
			}
		}

		log := pfxlog.ChannelLogger("policyEval", ctx.rolesSymbol.GetStore().GetSingularEntityType()+"Eval").
			WithFields(logrus.Fields{
				"id":       string(policyId),
				"semantic": semantic,
				"symbol":   ctx.rolesSymbol.GetName(),
			})
		evaluatePolicyAgainstEntity(ctx, semantic, entityId, policyId, ids, roles, entityRoles, log)
	}

	ctx.processServicePolicyEvents()
}

func EvaluatePolicy(ctx *roleAttributeChangeContext, policy Policy, roleAttributesSymbol boltz.EntitySetSymbol) {
	policyId := []byte(policy.GetId())
	_, semanticB := ctx.rolesSymbol.GetStore().GetSymbol(FieldSemantic).Eval(ctx.tx(), policyId)
	semantic := string(semanticB)
	if !isSemanticValid(semantic) {
		ctx.SetError(errors.Errorf("unable to get valid semantic for %v with %v, value found: %v",
			ctx.rolesSymbol.GetStore().GetSingularEntityType(), policy.GetId(), semantic))
	}

	log := pfxlog.ChannelLogger("policyEval", ctx.rolesSymbol.GetStore().GetSingularEntityType()+"Eval").
		WithFields(logrus.Fields{
			"id":       policy.GetId(),
			"semantic": semantic,
			"symbol":   ctx.rolesSymbol.GetName(),
		})

	roleSet := ctx.rolesSymbol.EvalStringList(ctx.tx(), policyId)
	roles, ids, err := splitRolesAndIds(roleSet)
	log.Tracef("roleSet: %v", roleSet)
	if err != nil {
		ctx.SetError(err)
		return
	}
	log.Tracef("roles: %v", roles)
	log.Tracef("ids: %v", ids)

	if err := validateEntityIds(ctx.tx(), ctx.linkCollection.GetLinkedSymbol().GetStore(), ctx.rolesSymbol.GetName(), ids); err != nil {
		ctx.SetError(err)
		return
	}

	cursor := roleAttributesSymbol.GetStore().IterateIds(ctx.tx(), ast.BoolNodeTrue)
	for ; cursor.IsValid(); cursor.Next() {
		entityId := cursor.Current()
		entityRoleAttributes := roleAttributesSymbol.EvalStringList(ctx.tx(), entityId)
		match, change := evaluatePolicyAgainstEntity(ctx, semantic, entityId, policyId, ids, roles, entityRoleAttributes, log)
		log.Tracef("evaluating %v match: %v, change: %v", string(entityId), match, change)
	}
	ctx.processServicePolicyEvents()
}

func validateEntityIds(tx *bbolt.Tx, store boltz.Store, field string, ids []string) error {
	var invalid []string
	for _, val := range ids {
		if !store.IsEntityPresent(tx, val) {
			invalid = append(invalid, val)
		}
	}
	if len(invalid) > 0 {
		return errorz.NewFieldError(fmt.Sprintf("no %v found with the given ids", store.GetEntityType()), field, invalid)
	}
	return nil
}

func UpdateRelatedRoles(ctx *roleAttributeChangeContext, entityId []byte, newRoleAttributes []boltz.FieldTypeAndValue, semanticSymbol boltz.EntitySymbol) {
	cursor := ctx.rolesSymbol.GetStore().IterateIds(ctx.tx(), ast.BoolNodeTrue)

	entityRoles := FieldValuesToIds(newRoleAttributes)

	for ; cursor.IsValid(); cursor.Next() {
		policyId := cursor.Current()
		roleSet := ctx.rolesSymbol.EvalStringList(ctx.tx(), policyId)
		roles, ids, err := splitRolesAndIds(roleSet)
		if err != nil {
			ctx.SetError(err)
			return
		}

		semantic := SemanticAllOf
		if _, semanticValue := semanticSymbol.Eval(ctx.tx(), policyId); semanticValue != nil {
			semantic = string(semanticValue)
		}

		log := pfxlog.ChannelLogger("policyEval", ctx.rolesSymbol.GetStore().GetSingularEntityType()+"Eval").
			WithFields(logrus.Fields{
				"id":       string(policyId),
				"semantic": semantic,
				"symbol":   ctx.rolesSymbol.GetName(),
			})
		evaluatePolicyAgainstEntity(ctx, semantic, entityId, policyId, ids, roles, entityRoles, log)
	}

	ctx.processServicePolicyEvents()
}

func evaluatePolicyAgainstEntity(ctx *roleAttributeChangeContext, semantic string, entityId, policyId []byte, ids, roles, roleAttributes []string, log *logrus.Entry) (bool, bool) {
	if stringz.Contains(ids, string(entityId)) || stringz.Contains(roles, "all") ||
		(strings.EqualFold(semantic, SemanticAllOf) && len(roles) > 0 && stringz.ContainsAll(roleAttributes, roles...)) ||
		(strings.EqualFold(semantic, SemanticAnyOf) && len(roles) > 0 && stringz.ContainsAny(roleAttributes, roles...)) {
		return true, ProcessEntityPolicyMatched(ctx, entityId, policyId, log)
	} else {
		return false, ProcessEntityPolicyUnmatched(ctx, entityId, policyId, log)
	}
}

func ProcessEntityPolicyMatched(ctx *roleAttributeChangeContext, entityId, policyId []byte, log *logrus.Entry) bool {
	// first add it to the denormalize link table from the policy to the entity (ex: service policy -> identity)
	// If it's already there (in other words, this policy didn't change in relation to the entity,
	// we don't have any further work to do
	if added, err := ctx.linkCollection.AddLink(ctx.tx(), policyId, entityId); ctx.SetError(err) || !added {
		return false
	}

	if ctx.changeHandler != nil {
		log.Tracef("change add handler called for entity %s", entityId)
		ctx.changeHandler(policyId, entityId, true)
	}

	// next iterate over the denormalized link tables going from entity to entity (ex: service -> identity)
	// If we were added to a policy, we need to update all the link tables for all the entities on the
	// other side of the policy. If we're the first link, we get added to the link table, otherwise we
	// increment the count of policies linking these entities
	cursor := ctx.relatedLinkCollection.IterateLinks(ctx.tx(), policyId)
	for ; cursor.IsValid(); cursor.Next() {
		relatedEntityId := cursor.Current()
		newCount, err := ctx.denormLinkCollection.IncrementLinkCount(ctx.tx(), entityId, relatedEntityId)
		if ctx.SetError(err) {
			return false
		}
		if ctx.denormChangeHandler != nil && newCount == 1 {
			log.Tracef("denorm change add handler called for entity %s -> related entity %s", entityId, relatedEntityId)
			ctx.denormChangeHandler(entityId, relatedEntityId, true)
		}
	}
	return true
}

func ProcessEntityPolicyUnmatched(ctx *roleAttributeChangeContext, entityId, policyId []byte, log *logrus.Entry) bool {
	// first remove it from the denormalize link table from the policy to the entity (ex: service policy -> identity)
	// If wasn't there (in other words, this policy didn't change in relation to the entity, we don't have any further work to do
	if removed, err := ctx.linkCollection.RemoveLink(ctx.tx(), policyId, entityId); ctx.SetError(err) || !removed {
		return false
	}

	if ctx.changeHandler != nil {
		log.Tracef("change remove handler called for entity %s", entityId)
		ctx.changeHandler(policyId, entityId, false)
	}

	// next iterate over the denormalized link tables going from entity to entity (ex: service -> identity)
	// If we were remove from a policy, we need to update all the link tables for all the entities on the
	// other side of the policy. If we're the last link, we get removed from the link table, otherwise we
	// decrement the count of policies linking these entities
	cursor := ctx.relatedLinkCollection.IterateLinks(ctx.tx(), policyId)
	for ; cursor.IsValid(); cursor.Next() {
		relatedEntityId := cursor.Current()
		newCount, err := ctx.denormLinkCollection.DecrementLinkCount(ctx.tx(), entityId, relatedEntityId)
		if ctx.SetError(err) {
			return false
		}
		if ctx.denormChangeHandler != nil && newCount == 0 {
			log.Tracef("denorm change remove handler called for entity %s -> related entity %s", entityId, relatedEntityId)
			ctx.denormChangeHandler(entityId, relatedEntityId, false)
		}
	}
	return true
}

type denormCheckCtx struct {
	name                   string
	mutateCtx              boltz.MutateContext
	sourceStore            boltz.Store
	targetStore            boltz.Store
	policyStore            boltz.Store
	sourceCollection       boltz.LinkCollection
	targetCollection       boltz.LinkCollection
	targetDenormCollection boltz.RefCountedLinkCollection
	policyFilter           func(policyId []byte) bool
	errorSink              func(err error, fixed bool)
	repair                 bool
}

func validatePolicyDenormalization(ctx *denormCheckCtx) error {
	tx := ctx.mutateCtx.Tx()

	links := map[string]map[string]int{}

	for policyCursor := ctx.policyStore.IterateIds(tx, ast.BoolNodeTrue); policyCursor.IsValid(); policyCursor.Next() {
		policyId := policyCursor.Current()
		if ctx.policyFilter == nil || ctx.policyFilter(policyId) {
			for sourceCursor := ctx.sourceCollection.IterateLinks(tx, policyId); sourceCursor.IsValid(); sourceCursor.Next() {
				sourceId := string(sourceCursor.Current())
				for destCursor := ctx.targetCollection.IterateLinks(tx, policyId); destCursor.IsValid(); destCursor.Next() {
					destId := string(destCursor.Current())
					destMap := links[sourceId]
					if destMap == nil {
						destMap = map[string]int{}
						links[sourceId] = destMap
					}
					destMap[destId] = destMap[destId] + 1
				}
			}
		}
	}

	for sourceCursor := ctx.sourceStore.IterateIds(tx, ast.BoolNodeTrue); sourceCursor.IsValid(); sourceCursor.Next() {
		sourceEntityId := sourceCursor.Current()
		for targetCursor := ctx.targetStore.IterateIds(tx, ast.BoolNodeTrue); targetCursor.IsValid(); targetCursor.Next() {
			targetEntityId := targetCursor.Current()

			linkCount := 0
			if destMap, ok := links[string(sourceEntityId)]; ok {
				linkCount = destMap[string(targetEntityId)]
			}
			var sourceLinkCount, targetLinkCount *int32
			var err error
			if ctx.repair {
				sourceLinkCount, targetLinkCount, err = ctx.targetDenormCollection.SetLinkCount(tx, sourceEntityId, targetEntityId, linkCount)
			} else {
				sourceLinkCount, targetLinkCount = ctx.targetDenormCollection.GetLinkCounts(tx, sourceEntityId, targetEntityId)
			}
			if err != nil {
				return err
			}
			logDiscrepencies(ctx, linkCount, sourceEntityId, targetEntityId, sourceLinkCount, targetLinkCount)
		}
	}
	return nil
}

func logDiscrepencies(ctx *denormCheckCtx, count int, sourceId, targetId []byte, sourceLinkCount, targetLinkCount *int32) {
	oldValuesMatch := (sourceLinkCount == nil && targetLinkCount == nil) || (sourceLinkCount != nil && targetLinkCount != nil && *sourceLinkCount == *targetLinkCount)
	if !oldValuesMatch {
		err := errors.Errorf("%v: ismatched link counts. %v %v (%v) <-> %v %v (%v), should be both are %v", ctx.name,
			ctx.sourceStore.GetSingularEntityType(), string(sourceId), sourceLinkCount,
			ctx.targetStore.GetSingularEntityType(), string(targetId), targetLinkCount, count)
		ctx.errorSink(err, ctx.repair)
	}

	if ((sourceLinkCount == nil || *sourceLinkCount == 0) && count != 0) ||
		(sourceLinkCount != nil && *sourceLinkCount != int32(count)) {
		sourceCount := int32(0)
		if sourceLinkCount != nil {
			sourceCount = *sourceLinkCount
		}
		err := errors.Errorf("%v: incorrect link counts for %v %v <-> %v %v is %v, should be %v", ctx.name,
			ctx.sourceStore.GetSingularEntityType(), string(sourceId),
			ctx.targetStore.GetSingularEntityType(), string(targetId),
			sourceCount, count)
		ctx.errorSink(err, ctx.repair)
	}
}
