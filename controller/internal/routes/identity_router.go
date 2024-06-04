/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package routes

import (
	"fmt"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/edge-api/rest_management_api_server/operations/identity"
	"github.com/openziti/edge-api/rest_model"
	"github.com/openziti/foundation/v2/errorz"
	"github.com/openziti/foundation/v2/stringz"
	"github.com/openziti/storage/ast"
	"github.com/openziti/storage/boltz"
	"github.com/openziti/ziti/common/logcontext"
	"github.com/openziti/ziti/controller/db"
	"github.com/openziti/ziti/controller/env"
	"github.com/openziti/ziti/controller/fields"
	"github.com/openziti/ziti/controller/internal/permissions"
	"github.com/openziti/ziti/controller/model"
	"github.com/openziti/ziti/controller/models"
	"github.com/openziti/ziti/controller/response"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

func init() {
	r := NewIdentityRouter()
	env.AddRouter(r)
}

type IdentityRouter struct {
	BasePath string
}

func NewIdentityRouter() *IdentityRouter {
	return &IdentityRouter{
		BasePath: "/" + EntityNameIdentity,
	}
}

func (r *IdentityRouter) Register(ae *env.AppEnv) {

	//identity crud
	ae.ManagementApi.IdentityDeleteIdentityHandler = identity.DeleteIdentityHandlerFunc(func(params identity.DeleteIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.Delete, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityDetailIdentityHandler = identity.DetailIdentityHandlerFunc(func(params identity.DetailIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.Detail, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityListIdentitiesHandler = identity.ListIdentitiesHandlerFunc(func(params identity.ListIdentitiesParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.List, params.HTTPRequest, "", "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityUpdateIdentityHandler = identity.UpdateIdentityHandlerFunc(func(params identity.UpdateIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Update(ae, rc, params) }, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityCreateIdentityHandler = identity.CreateIdentityHandlerFunc(func(params identity.CreateIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Create(ae, rc, params) }, params.HTTPRequest, "", "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityPatchIdentityHandler = identity.PatchIdentityHandlerFunc(func(params identity.PatchIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Patch(ae, rc, params) }, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// authenticators list
	ae.ManagementApi.IdentityGetIdentityAuthenticatorsHandler = identity.GetIdentityAuthenticatorsHandlerFunc(func(params identity.GetIdentityAuthenticatorsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listAuthenticators, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// enrollments list
	ae.ManagementApi.IdentityGetIdentityEnrollmentsHandler = identity.GetIdentityEnrollmentsHandlerFunc(func(params identity.GetIdentityEnrollmentsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listEnrollments, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// edge router policies list
	ae.ManagementApi.IdentityListIdentitysEdgeRouterPoliciesHandler = identity.ListIdentitysEdgeRouterPoliciesHandlerFunc(func(params identity.ListIdentitysEdgeRouterPoliciesParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listEdgeRouterPolicies, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// edge routers list
	ae.ManagementApi.IdentityListIdentityEdgeRoutersHandler = identity.ListIdentityEdgeRoutersHandlerFunc(func(params identity.ListIdentityEdgeRoutersParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listEdgeRouters, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// service policies list
	ae.ManagementApi.IdentityListIdentityServicePoliciesHandler = identity.ListIdentityServicePoliciesHandlerFunc(func(params identity.ListIdentityServicePoliciesParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listServicePolicies, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// service list
	ae.ManagementApi.IdentityListIdentityServicesHandler = identity.ListIdentityServicesHandlerFunc(func(params identity.ListIdentityServicesParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.listServices(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// service configs crud
	ae.ManagementApi.IdentityListIdentitysServiceConfigsHandler = identity.ListIdentitysServiceConfigsHandlerFunc(func(params identity.ListIdentitysServiceConfigsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.listServiceConfigs, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityAssociateIdentitysServiceConfigsHandler = identity.AssociateIdentitysServiceConfigsHandlerFunc(func(params identity.AssociateIdentitysServiceConfigsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.assignServiceConfigs(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityDisassociateIdentitysServiceConfigsHandler = identity.DisassociateIdentitysServiceConfigsHandlerFunc(func(params identity.DisassociateIdentitysServiceConfigsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.removeServiceConfigs(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// policy advice URL
	ae.ManagementApi.IdentityGetIdentityPolicyAdviceHandler = identity.GetIdentityPolicyAdviceHandlerFunc(func(params identity.GetIdentityPolicyAdviceParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.getPolicyAdvice, params.HTTPRequest, params.ID, params.ServiceID, permissions.IsAdmin())
	})

	// posture data
	ae.ManagementApi.IdentityGetIdentityPostureDataHandler = identity.GetIdentityPostureDataHandlerFunc(func(params identity.GetIdentityPostureDataParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.getPostureData, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityGetIdentityFailedServiceRequestsHandler = identity.GetIdentityFailedServiceRequestsHandlerFunc(func(params identity.GetIdentityFailedServiceRequestsParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.getPostureDataFailedServiceRequests, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// mfa
	ae.ManagementApi.IdentityRemoveIdentityMfaHandler = identity.RemoveIdentityMfaHandlerFunc(func(params identity.RemoveIdentityMfaParams, i interface{}) middleware.Responder {
		return ae.IsAllowed(r.removeMfa, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// trace
	ae.ManagementApi.IdentityUpdateIdentityTracingHandler = identity.UpdateIdentityTracingHandlerFunc(func(params identity.UpdateIdentityTracingParams, i interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.updateTracing(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	// disable / enable
	ae.ManagementApi.IdentityEnableIdentityHandler = identity.EnableIdentityHandlerFunc(func(params identity.EnableIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.Enable(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.IdentityDisableIdentityHandler = identity.DisableIdentityHandlerFunc(func(params identity.DisableIdentityParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) {
			r.Disable(ae, rc, params)
		}, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})
}

func (r *IdentityRouter) List(ae *env.AppEnv, rc *response.RequestContext) {
	roleFilters := rc.Request.URL.Query()["roleFilter"]
	roleSemantic := rc.Request.URL.Query().Get("roleSemantic")

	if len(roleFilters) > 0 {
		ListWithQueryF[*model.Identity](ae, rc, ae.Managers.Identity, MapIdentityToRestEntity, func(query ast.Query) (*models.EntityListResult[*model.Identity], error) {
			cursorProvider, err := ae.GetStores().Identity.GetRoleAttributesCursorProvider(roleFilters, roleSemantic)
			if err != nil {
				return nil, err
			}
			return ae.Managers.Identity.BasePreparedListIndexed(cursorProvider, query)
		})
	} else {
		ListWithHandler[*model.Identity](ae, rc, ae.Managers.Identity, MapIdentityToRestEntity)
	}
}

func (r *IdentityRouter) Detail(ae *env.AppEnv, rc *response.RequestContext) {
	DetailWithHandler[*model.Identity](ae, rc, ae.Managers.Identity, MapIdentityToRestEntity)
}

func getIdentityTypeId(ae *env.AppEnv, identityType rest_model.IdentityType) string {
	//todo: Remove this, should be identityTypeId coming in through the API so we can defer this lookup and subsequent checks to the handlers
	if identityType == rest_model.IdentityTypeDevice || identityType == rest_model.IdentityTypeService || identityType == rest_model.IdentityTypeUser {
		return db.DefaultIdentityType
	}
	identityTypeId := ""
	if identityType, err := ae.Managers.IdentityType.ReadByName(string(identityType)); identityType != nil && err == nil {
		identityTypeId = identityType.Id
	}

	return identityTypeId
}

func (r *IdentityRouter) Create(ae *env.AppEnv, rc *response.RequestContext, params identity.CreateIdentityParams) {
	Create(rc, rc, IdentityLinkFactory, func() (string, error) {
		identityModel, enrollments := MapCreateIdentityToModel(params.Identity, getIdentityTypeId(ae, *params.Identity.Type))
		err := ae.Managers.Identity.CreateWithEnrollments(identityModel, enrollments, rc.NewChangeContext())
		if err != nil {
			return "", err
		}
		return identityModel.Id, nil
	})
}

func (r *IdentityRouter) Delete(ae *env.AppEnv, rc *response.RequestContext) {
	DeleteWithHandler(rc, ae.Managers.Identity)
}

func (r *IdentityRouter) Update(ae *env.AppEnv, rc *response.RequestContext, params identity.UpdateIdentityParams) {
	Update(rc, func(id string) error {
		return ae.Managers.Identity.Update(MapUpdateIdentityToModel(params.ID, params.Identity, getIdentityTypeId(ae, *params.Identity.Type)), nil, rc.NewChangeContext())
	})
}

func (r *IdentityRouter) Patch(ae *env.AppEnv, rc *response.RequestContext, params identity.PatchIdentityParams) {
	Patch(rc, func(id string, fields fields.UpdatedFields) error {
		fields = fields.FilterMaps(boltz.FieldTags, db.FieldIdentityAppData, db.FieldIdentityServiceHostingCosts, db.FieldIdentityServiceHostingPrecedences)
		return ae.Managers.Identity.Update(MapPatchIdentityToModel(params.ID, params.Identity, getIdentityTypeId(ae, params.Identity.Type)), fields, rc.NewChangeContext())
	})
}

func (r *IdentityRouter) listEdgeRouterPolicies(ae *env.AppEnv, rc *response.RequestContext) {
	ListAssociationWithHandler[*model.Identity, *model.EdgeRouterPolicy](ae, rc, ae.Managers.Identity, ae.Managers.EdgeRouterPolicy, MapEdgeRouterPolicyToRestEntity)
}

func (r *IdentityRouter) listServicePolicies(ae *env.AppEnv, rc *response.RequestContext) {
	ListAssociationWithHandler[*model.Identity, *model.ServicePolicy](ae, rc, ae.Managers.Identity, ae.Managers.ServicePolicy, MapServicePolicyToRestEntity)
}

func (r *IdentityRouter) listServices(ae *env.AppEnv, rc *response.RequestContext, params identity.ListIdentityServicesParams) {
	typeFilter := ""
	if params.PolicyType != nil {
		if strings.EqualFold(*params.PolicyType, db.PolicyTypeBind.String()) {
			typeFilter = fmt.Sprintf(` and type = %d`, db.PolicyTypeBind.Id())
		}

		if strings.EqualFold(*params.PolicyType, db.PolicyTypeDial.String()) {
			typeFilter = fmt.Sprintf(` and type = %d`, db.PolicyTypeDial.Id())
		}
	}

	filterTemplate := `not isEmpty(from servicePolicies where anyOf(identities) = "%v"` + typeFilter + ")"
	ListAssociationsWithFilter[*model.ServiceDetail](ae, rc, filterTemplate, ae.Managers.EdgeService.GetDetailLister(), MapServiceToRestEntity)
}

func (r *IdentityRouter) listAuthenticators(ae *env.AppEnv, rc *response.RequestContext) {
	filterTemplate := `identity = "%v"`
	ListAssociationsWithFilter[*model.Authenticator](ae, rc, filterTemplate, ae.Managers.Authenticator, MapAuthenticatorToRestEntity)
}

func (r *IdentityRouter) listEnrollments(ae *env.AppEnv, rc *response.RequestContext) {
	filterTemplate := `identity = "%v"`
	ListAssociationsWithFilter[*model.Enrollment](ae, rc, filterTemplate, ae.Managers.Enrollment, MapEnrollmentToRestEntity)
}

func (r *IdentityRouter) listEdgeRouters(ae *env.AppEnv, rc *response.RequestContext) {
	filterTemplate := `not isEmpty(from edgeRouterPolicies where anyOf(identities) = "%v")`
	ListAssociationsWithFilter[*model.EdgeRouter](ae, rc, filterTemplate, ae.Managers.EdgeRouter, MapEdgeRouterToRestEntity)
}

func (r *IdentityRouter) listServiceConfigs(ae *env.AppEnv, rc *response.RequestContext) {
	listWithId(rc, func(id string) ([]interface{}, error) {
		modelIdentity, err := ae.Managers.Identity.Read(id)
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, 0)
		for serviceId, configData := range modelIdentity.ServiceConfigs {
			service, err := ae.Managers.EdgeService.Read(serviceId)
			if err != nil {
				pfxlog.Logger().Debugf("listing service configs for identity [%s] could not find service [%s]: %v", id, serviceId, err)
				continue
			}

			for _, configId := range configData {
				config, err := ae.Managers.Config.Read(configId)
				if err != nil {
					pfxlog.Logger().Debugf("listing service configs for identity [%s] could not find config [%s]: %v", id, configId, err)
					continue
				}

				result = append(result, rest_model.ServiceConfigDetail{
					Config:    ToEntityRef(config.Name, config, ConfigLinkFactory),
					ConfigID:  &config.Id,
					Service:   ToEntityRef(service.Name, service, ServiceLinkFactory),
					ServiceID: &service.Id,
				})
			}
		}
		return result, nil
	})
}

func (r *IdentityRouter) assignServiceConfigs(ae *env.AppEnv, rc *response.RequestContext, params identity.AssociateIdentitysServiceConfigsParams) {
	Update(rc, func(id string) error {
		var modelServiceConfigs []model.ServiceConfig
		for _, serviceConfig := range params.ServiceConfigs {
			modelServiceConfigs = append(modelServiceConfigs, MapServiceConfigToModel(*serviceConfig))
		}
		return ae.Managers.Identity.AssignServiceConfigs(id, modelServiceConfigs, rc.NewChangeContext())
	})
}

func (r *IdentityRouter) removeServiceConfigs(ae *env.AppEnv, rc *response.RequestContext, params identity.DisassociateIdentitysServiceConfigsParams) {
	UpdateAllowEmptyBody(rc, func(id string) error {
		var modelServiceConfigs []model.ServiceConfig
		for _, serviceConfig := range params.ServiceConfigIDPairs {
			modelServiceConfigs = append(modelServiceConfigs, MapServiceConfigToModel(*serviceConfig))
		}
		return ae.Managers.Identity.RemoveServiceConfigs(id, modelServiceConfigs, rc.NewChangeContext())
	})
}

func (r *IdentityRouter) getPolicyAdvice(ae *env.AppEnv, rc *response.RequestContext) {
	id, err := rc.GetEntityId()

	if err != nil {
		log := pfxlog.Logger()
		logErr := fmt.Errorf("could not find id property: %v", response.IdPropertyName)
		log.WithField("property", response.IdPropertyName).Error(logErr)
		rc.RespondWithError(err)
		return
	}

	serviceId, err := rc.GetEntitySubId()

	if err != nil {
		log := pfxlog.Logger()
		logErr := fmt.Errorf("could not find subId property: %v", response.SubIdPropertyName)
		log.WithField("property", response.SubIdPropertyName).Error(logErr)
		rc.RespondWithError(err)
		return
	}

	result, err := ae.Managers.PolicyAdvisor.AnalyzeServiceReachability(id, serviceId)

	if err != nil {
		if boltz.IsErrNotFoundErr(err) {
			rc.RespondWithNotFoundWithCause(err)
			return
		}

		log := pfxlog.Logger()
		log.WithField("cause", err).Error("could not convert list")
		rc.RespondWithError(err)
		return
	}

	output := MapAdvisorServiceReachabilityToRestEntity(result)
	rc.RespondWithOk(output, nil)
}

func (r *IdentityRouter) getPostureData(ae *env.AppEnv, rc *response.RequestContext) {
	id, _ := rc.GetEntityId()
	postureData := ae.GetManagers().PostureResponse.PostureData(id)

	rc.RespondWithOk(MapPostureDataToRestModel(ae, postureData), &rest_model.Meta{})
}

func (r *IdentityRouter) getPostureDataFailedServiceRequests(ae *env.AppEnv, rc *response.RequestContext) {
	id, _ := rc.GetEntityId()
	postureData := ae.GetManagers().PostureResponse.PostureData(id)

	rc.RespondWithOk(MapPostureDataFailedSessionRequestToRestModel(postureData.SessionRequestFailures), &rest_model.Meta{})
}

func (r *IdentityRouter) removeMfa(ae *env.AppEnv, rc *response.RequestContext) {
	id, _ := rc.GetEntityId()
	err := ae.Managers.Mfa.DeleteAllForIdentity(id, rc.NewChangeContext())

	if err != nil {
		rc.RespondWithError(err)
		return
	}

	rc.RespondWithEmptyOk()
}

func (r *IdentityRouter) updateTracing(ae *env.AppEnv, rc *response.RequestContext, params identity.UpdateIdentityTracingParams) {
	id, _ := rc.GetEntityId()
	_, err := ae.Managers.Identity.Read(id)

	if err != nil {
		rc.RespondWithError(err)
		return
	}

	if params.TraceSpec.Enabled {
		d, err := time.ParseDuration(params.TraceSpec.Duration)
		if err != nil {
			rc.RespondWithError(errorz.NewFieldError(err.Error(), "duration", params.TraceSpec.Duration))
			return
		}

		if params.TraceSpec.TraceID == "" {
			params.TraceSpec.TraceID = uuid.NewString()
		}

		var channels []string
		if len(params.TraceSpec.Channels) == 0 || stringz.Contains(params.TraceSpec.Channels, "all") {
			channels = append(channels, logcontext.SelectPath, logcontext.EstablishPath)
		} else {
			channels = params.TraceSpec.Channels
		}

		var channelMask uint32
		for _, channel := range channels {
			channelMask |= logcontext.GetChannelMask(channel)
		}

		spec := ae.TraceManager.TraceIdentity(id, d, params.TraceSpec.TraceID, channelMask)
		logrus.Infof("enabling tracing for identity %v with traceId %v for %v with mask %v", id, params.TraceSpec.TraceID, d, channelMask)
		rc.RespondWithOk(&rest_model.TraceDetail{
			Enabled: true,
			TraceID: params.TraceSpec.TraceID,
			Until:   strfmt.DateTime(spec.Until),
		}, nil)
	} else {
		ae.TraceManager.RemoveIdentityTrace(id)
		rc.RespondWithOk(&rest_model.TraceDetail{
			Enabled: false,
		}, nil)
	}
}

func (r *IdentityRouter) Enable(ae *env.AppEnv, rc *response.RequestContext, params identity.EnableIdentityParams) {
	if err := ae.Managers.Identity.Enable(params.ID, rc.NewChangeContext()); err != nil {
		rc.RespondWithError(err)
		return
	}

	rc.RespondWithEmptyOk()
}

func (r *IdentityRouter) Disable(ae *env.AppEnv, rc *response.RequestContext, params identity.DisableIdentityParams) {
	if err := ae.Managers.Identity.Disable(params.ID, time.Duration(*params.Disable.DurationMinutes)*time.Minute, rc.NewChangeContext()); err != nil {
		rc.RespondWithError(err)
		return
	}
}
