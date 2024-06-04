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

package db

import (
	"fmt"
	"github.com/openziti/foundation/v2/errorz"
	"github.com/openziti/storage/ast"
	"github.com/openziti/storage/boltz"
	"github.com/openziti/ziti/common/eid"
	"go.etcd.io/bbolt"
)

const (
	FieldConfigData            = "data"
	FieldConfigType            = "type"
	FieldConfigIdentityService = "identityServices"
)

func newConfig(name string, configType string, data map[string]interface{}) *Config {
	return &Config{
		BaseExtEntity: boltz.BaseExtEntity{Id: eid.New()},
		Name:          name,
		Type:          configType,
		Data:          data,
	}
}

type Config struct {
	boltz.BaseExtEntity
	Name string                 `json:"name"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func (entity *Config) GetName() string {
	return entity.Name
}

func (entity *Config) GetEntityType() string {
	return EntityTypeConfigs
}

var _ ConfigStore = (*configStoreImpl)(nil)

type ConfigStore interface {
	Store[*Config]
	NameIndexed
}

func newConfigsStore(stores *stores) *configStoreImpl {
	store := &configStoreImpl{}
	store.baseStore = newBaseStore[*Config](stores, store)
	store.InitImpl(store)
	return store
}

type configStoreImpl struct {
	*baseStore[*Config]

	indexName              boltz.ReadIndex
	symbolType             boltz.EntitySymbol
	symbolServices         boltz.EntitySetSymbol
	symbolIdentityServices boltz.EntitySetSymbol
	identityServicesLinks  *boltz.LinkedSetSymbol
}

func (store *configStoreImpl) GetNameIndex() boltz.ReadIndex {
	return store.indexName
}

func (store *configStoreImpl) initializeLocal() {
	store.AddExtEntitySymbols()
	store.indexName = store.addUniqueNameField()
	store.symbolType = store.AddFkSymbol(FieldConfigType, store.stores.configType)
	store.AddMapSymbol(FieldConfigData, ast.NodeTypeAnyType, FieldConfigData)
	store.symbolServices = store.AddFkSetSymbol(EntityTypeServices, store.stores.edgeService)
	store.symbolIdentityServices = store.AddSetSymbol(FieldConfigIdentityService, ast.NodeTypeOther)
	store.identityServicesLinks = &boltz.LinkedSetSymbol{EntitySymbol: store.symbolIdentityServices}
}

func (store *configStoreImpl) initializeLinked() {
	store.AddFkIndex(store.symbolType, store.stores.configType.symbolConfigs)
	store.AddLinkCollection(store.symbolServices, store.stores.edgeService.symbolConfigs)
}

func (store *configStoreImpl) NewEntity() *Config {
	return &Config{}
}

func (store *configStoreImpl) FillEntity(entity *Config, bucket *boltz.TypedBucket) {
	entity.LoadBaseValues(bucket)
	entity.Name = bucket.GetStringOrError(FieldName)
	entity.Type = bucket.GetStringOrError(FieldConfigType)
	entity.Data = bucket.GetMap(FieldConfigData)
}

func (store *configStoreImpl) PersistEntity(entity *Config, ctx *boltz.PersistContext) {
	entity.SetBaseValues(ctx)
	ctx.SetString(FieldName, entity.Name)
	ctx.SetString(FieldConfigType, entity.Type)
	ctx.SetMap(FieldConfigData, entity.Data)

	if ctx.ProceedWithSet(FieldConfigData) && entity.Data == nil {
		ctx.Bucket.SetError(errorz.NewFieldError("data is required", "data", nil))
	}
}

func (store *configStoreImpl) Update(ctx boltz.MutateContext, entity *Config, checker boltz.FieldChecker) error {
	if err := store.createServiceChangeEvents(ctx.Tx(), entity.GetId()); err != nil {
		return err
	}
	return store.baseStore.Update(ctx, entity, checker)
}

func (store *configStoreImpl) DeleteById(ctx boltz.MutateContext, id string) error {
	if err := store.createServiceChangeEvents(ctx.Tx(), id); err != nil {
		return err
	}

	err := store.symbolIdentityServices.Map(ctx.Tx(), []byte(id), func(mapCtx *boltz.MapContext) {
		keys, err := boltz.DecodeStringSlice(mapCtx.Value())
		if err != nil {
			mapCtx.SetError(err)
			return
		}
		identityId := keys[0]
		serviceId := keys[1]
		mapCtx.SetError(fmt.Errorf("config is in use by identity %s for service %s", identityId, serviceId))
	})
	if err != nil {
		return err
	}
	return store.baseStore.DeleteById(ctx, id)
}

func (store *configStoreImpl) createServiceChangeEvents(tx *bbolt.Tx, configId string) error {
	eh := &serviceEventHandler{}

	id := []byte(configId)
	err := store.symbolServices.Map(tx, id, func(ctx *boltz.MapContext) {
		eh.addServiceUpdatedEvent(store.stores, tx, ctx.Value())
	})

	if err != nil {
		return err
	}

	return store.symbolIdentityServices.Map(tx, id, func(mapCtx *boltz.MapContext) {
		keys, err := boltz.DecodeStringSlice(mapCtx.Value())
		if err != nil {
			mapCtx.SetError(err)
			return
		}
		identityId := keys[0]
		serviceId := keys[1]
		eh.addServiceEvent(tx, []byte(identityId), []byte(serviceId), ServiceUpdated)
	})
}
