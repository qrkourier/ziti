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
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime/middleware"
	"github.com/openziti/edge-api/rest_management_api_server/operations/config"
	"github.com/openziti/ziti/controller/env"
	"github.com/openziti/ziti/controller/fields"
	"github.com/openziti/ziti/controller/internal/permissions"
	"github.com/openziti/ziti/controller/model"
	"github.com/openziti/ziti/controller/response"
)

func init() {
	r := NewConfigTypeRouter()
	env.AddRouter(r)
}

type ConfigTypeRouter struct {
	BasePath string
}

func NewConfigTypeRouter() *ConfigTypeRouter {
	return &ConfigTypeRouter{
		BasePath: "/" + EntityNameConfigType,
	}
}

func (r *ConfigTypeRouter) Register(ae *env.AppEnv) {
	ae.ManagementApi.ConfigDeleteConfigTypeHandler = config.DeleteConfigTypeHandlerFunc(func(params config.DeleteConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.Delete, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigDetailConfigTypeHandler = config.DetailConfigTypeHandlerFunc(func(params config.DetailConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.Detail, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigListConfigTypesHandler = config.ListConfigTypesHandlerFunc(func(params config.ListConfigTypesParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(r.List, params.HTTPRequest, "", "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigUpdateConfigTypeHandler = config.UpdateConfigTypeHandlerFunc(func(params config.UpdateConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Update(ae, rc, params) }, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigCreateConfigTypeHandler = config.CreateConfigTypeHandlerFunc(func(params config.CreateConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Create(ae, rc, params) }, params.HTTPRequest, "", "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigPatchConfigTypeHandler = config.PatchConfigTypeHandlerFunc(func(params config.PatchConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.Patch(ae, rc, params) }, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})

	ae.ManagementApi.ConfigListConfigsForConfigTypeHandler = config.ListConfigsForConfigTypeHandlerFunc(func(params config.ListConfigsForConfigTypeParams, _ interface{}) middleware.Responder {
		return ae.IsAllowed(func(ae *env.AppEnv, rc *response.RequestContext) { r.ListConfigs(ae, rc) }, params.HTTPRequest, params.ID, "", permissions.IsAdmin())
	})
}

func (r *ConfigTypeRouter) List(ae *env.AppEnv, rc *response.RequestContext) {
	ListWithHandler[*model.ConfigType](ae, rc, ae.Managers.ConfigType, MapConfigTypeToRestEntity)
}

func (r *ConfigTypeRouter) Detail(ae *env.AppEnv, rc *response.RequestContext) {
	DetailWithHandler[*model.ConfigType](ae, rc, ae.Managers.ConfigType, MapConfigTypeToRestEntity)
}

func (r *ConfigTypeRouter) Create(ae *env.AppEnv, rc *response.RequestContext, params config.CreateConfigTypeParams) {
	if params.ConfigType.Schema != nil {
		if _, ok := params.ConfigType.Schema.(map[string]interface{}); !ok {
			ae.ManagementApi.ServeErrorFor("")(rc.ResponseWriter, rc.Request, errors.InvalidType("schema", "body", "object", params.ConfigType.Schema))
			return
		}
	}

	Create(rc, rc, ConfigTypeLinkFactory, func() (string, error) {
		return MapCreate(ae.Managers.ConfigType.Create, MapCreateConfigTypeToModel(params.ConfigType), rc)
	})
}

func (r *ConfigTypeRouter) Delete(ae *env.AppEnv, rc *response.RequestContext) {
	DeleteWithHandler(rc, ae.Managers.ConfigType)
}

func (r *ConfigTypeRouter) Update(ae *env.AppEnv, rc *response.RequestContext, params config.UpdateConfigTypeParams) {
	if params.ConfigType.Schema != nil {
		if _, ok := params.ConfigType.Schema.(map[string]interface{}); !ok {
			ae.ManagementApi.ServeErrorFor("")(rc.ResponseWriter, rc.Request, errors.InvalidType("schema", "body", "object", params.ConfigType.Schema))
			return
		}
	}

	Update(rc, func(id string) error {
		return ae.Managers.ConfigType.Update(MapUpdateConfigTypeToModel(params.ID, params.ConfigType), nil, rc.NewChangeContext())
	})
}

func (r *ConfigTypeRouter) Patch(ae *env.AppEnv, rc *response.RequestContext, params config.PatchConfigTypeParams) {
	if _, ok := params.ConfigType.Schema.(map[string]interface{}); !ok {
		ae.ManagementApi.ServeErrorFor("")(rc.ResponseWriter, rc.Request, errors.InvalidType("schema", "body", "object", params.ConfigType.Schema))
		return
	}
	if params.ConfigType.Schema == nil {
		ae.ManagementApi.ServeErrorFor("")(rc.ResponseWriter, rc.Request, errors.Required("schema", "body", nil))
		return
	}

	Patch(rc, func(id string, fields fields.UpdatedFields) error {
		return ae.Managers.ConfigType.Update(MapPatchConfigTypeToModel(params.ID, params.ConfigType), fields.FilterMaps("tags", "schema"), rc.NewChangeContext())
	})
}

func (r *ConfigTypeRouter) ListConfigs(ae *env.AppEnv, rc *response.RequestContext) {
	ListAssociationWithHandler[*model.ConfigType, *model.Config](ae, rc, ae.Managers.ConfigType, ae.Managers.Config, MapConfigToRestEntity)
}
