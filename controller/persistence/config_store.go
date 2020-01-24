/*
	Copyright 2020 Netfoundry, Inc.

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

package persistence

import (
	"github.com/google/uuid"
	"github.com/netfoundry/ziti-foundation/storage/ast"
	"github.com/netfoundry/ziti-foundation/storage/boltz"
	"go.etcd.io/bbolt"
)

const (
	FieldConfigData = "data"
	FieldConfigType = "type"
)

func newConfig(name string, configType string, data map[string]interface{}) *Config {
	return &Config{
		BaseEdgeEntityImpl: BaseEdgeEntityImpl{Id: uuid.New().String()},
		Name:               name,
		Type:               configType,
		Data:               data,
	}
}

type Config struct {
	BaseEdgeEntityImpl
	Name string
	Type string
	Data map[string]interface{}
}

func (entity *Config) LoadValues(_ boltz.CrudStore, bucket *boltz.TypedBucket) {
	entity.LoadBaseValues(bucket)
	entity.Name = bucket.GetStringOrError(FieldName)
	entity.Type = bucket.GetStringOrError(FieldConfigType)
	entity.Data = bucket.GetMap(FieldConfigData)
}

func (entity *Config) SetValues(ctx *boltz.PersistContext) {
	entity.SetBaseValues(ctx)
	ctx.SetString(FieldName, entity.Name)
	ctx.SetString(FieldConfigType, entity.Type)
	ctx.SetMap(FieldConfigData, entity.Data)
}

func (entity *Config) GetEntityType() string {
	return EntityTypeConfigs
}

type ConfigStore interface {
	Store
	LoadOneById(tx *bbolt.Tx, id string) (*Config, error)
	LoadOneByName(tx *bbolt.Tx, name string) (*Config, error)
	GetNameIndex() boltz.ReadIndex
}

func newConfigsStore(stores *stores) *configStoreImpl {
	store := &configStoreImpl{
		baseStore: newBaseStore(stores, EntityTypeConfigs),
	}
	store.InitImpl(store)
	return store
}

type configStoreImpl struct {
	*baseStore

	indexName      boltz.ReadIndex
	symbolType     boltz.EntitySymbol
	symbolServices boltz.EntitySetSymbol
}

func (store *configStoreImpl) GetNameIndex() boltz.ReadIndex {
	return store.indexName
}

func (store *configStoreImpl) initializeLocal() {
	store.addBaseFields()
	store.indexName = store.addUniqueNameField()
	store.symbolType = store.AddFkSymbol(FieldConfigType, store.stores.configType)
	store.AddMapSymbol(FieldConfigData, ast.NodeTypeAnyType, FieldConfigData)
	store.symbolServices = store.AddFkSetSymbol(EntityTypeServices, store.stores.edgeService)
}

func (store *configStoreImpl) initializeLinked() {
	store.AddFkIndex(store.symbolType, store.stores.configType.symbolConfigs)
	store.AddLinkCollection(store.symbolServices, store.stores.edgeService.symbolConfigs)
}

func (store *configStoreImpl) NewStoreEntity() boltz.BaseEntity {
	return &Config{}
}

func (store *configStoreImpl) LoadOneById(tx *bbolt.Tx, id string) (*Config, error) {
	entity := &Config{}
	if err := store.baseLoadOneById(tx, id, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

func (store *configStoreImpl) LoadOneByName(tx *bbolt.Tx, name string) (*Config, error) {
	id := store.indexName.Read(tx, []byte(name))
	if id != nil {
		return store.LoadOneById(tx, string(id))
	}
	return nil, nil
}
