// Code generated by go-swagger; DO NOT EDIT.

//
// Copyright NetFoundry, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// __          __              _
// \ \        / /             (_)
//  \ \  /\  / /_ _ _ __ _ __  _ _ __   __ _
//   \ \/  \/ / _` | '__| '_ \| | '_ \ / _` |
//    \  /\  / (_| | |  | | | | | | | | (_| | : This file is generated, do not edit it.
//     \/  \/ \__,_|_|  |_| |_|_|_| |_|\__, |
//                                      __/ |
//                                     |___/

package service

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// New creates a new service API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) ClientService {
	return &Client{transport: transport, formats: formats}
}

/*
Client for service API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

// ClientService is the interface for Client methods
type ClientService interface {
	CreateService(params *CreateServiceParams, authInfo runtime.ClientAuthInfoWriter) (*CreateServiceOK, error)

	DeleteService(params *DeleteServiceParams, authInfo runtime.ClientAuthInfoWriter) (*DeleteServiceOK, error)

	DetailService(params *DetailServiceParams, authInfo runtime.ClientAuthInfoWriter) (*DetailServiceOK, error)

	ListServiceConfig(params *ListServiceConfigParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceConfigOK, error)

	ListServiceEdgeRouters(params *ListServiceEdgeRoutersParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceEdgeRoutersOK, error)

	ListServiceIdentities(params *ListServiceIdentitiesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceIdentitiesOK, error)

	ListServiceServiceEdgeRouterPolicies(params *ListServiceServiceEdgeRouterPoliciesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceServiceEdgeRouterPoliciesOK, error)

	ListServiceServicePolicies(params *ListServiceServicePoliciesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceServicePoliciesOK, error)

	ListServiceTerminators(params *ListServiceTerminatorsParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceTerminatorsOK, error)

	ListServices(params *ListServicesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServicesOK, error)

	PatchService(params *PatchServiceParams, authInfo runtime.ClientAuthInfoWriter) (*PatchServiceOK, error)

	UpdateService(params *UpdateServiceParams, authInfo runtime.ClientAuthInfoWriter) (*UpdateServiceOK, error)

	SetTransport(transport runtime.ClientTransport)
}

/*
  CreateService creates a services resource

  Create a services resource. Requires admin access.
*/
func (a *Client) CreateService(params *CreateServiceParams, authInfo runtime.ClientAuthInfoWriter) (*CreateServiceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateServiceParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "createService",
		Method:             "POST",
		PathPattern:        "/services",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &CreateServiceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateServiceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for createService: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  DeleteService deletes a service

  Delete a service by id. Requires admin access.
*/
func (a *Client) DeleteService(params *DeleteServiceParams, authInfo runtime.ClientAuthInfoWriter) (*DeleteServiceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeleteServiceParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "deleteService",
		Method:             "DELETE",
		PathPattern:        "/services/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &DeleteServiceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DeleteServiceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for deleteService: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  DetailService retrieves a single service

  Retrieves a single service by id. Requires admin access.
*/
func (a *Client) DetailService(params *DetailServiceParams, authInfo runtime.ClientAuthInfoWriter) (*DetailServiceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDetailServiceParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "detailService",
		Method:             "GET",
		PathPattern:        "/services/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &DetailServiceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DetailServiceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for detailService: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceConfig lists configs associated to a specific service

  Retrieves a list of config resources associated to a specific service; supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServiceConfig(params *ListServiceConfigParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceConfigOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceConfigParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceConfig",
		Method:             "GET",
		PathPattern:        "/services/{id}/configs",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceConfigReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceConfigOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceConfig: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceEdgeRouters lists accessible edge routers

  Retrieves a list of edge-routers that may be used to access the given service. Supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServiceEdgeRouters(params *ListServiceEdgeRoutersParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceEdgeRoutersOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceEdgeRoutersParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceEdgeRouters",
		Method:             "GET",
		PathPattern:        "/services/{id}/edge-routers",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceEdgeRoutersReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceEdgeRoutersOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceEdgeRouters: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceIdentities lists identities with access

  Retrieves a list of identities that have access to this service. Supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServiceIdentities(params *ListServiceIdentitiesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceIdentitiesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceIdentitiesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceIdentities",
		Method:             "GET",
		PathPattern:        "/services/{id}/identities",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceIdentitiesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceIdentitiesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceIdentities: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceServiceEdgeRouterPolicies lists service edge router policies that affect a specific service

  Retrieves a list of service edge router policy resources that affect a specific service; supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServiceServiceEdgeRouterPolicies(params *ListServiceServiceEdgeRouterPoliciesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceServiceEdgeRouterPoliciesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceServiceEdgeRouterPoliciesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceServiceEdgeRouterPolicies",
		Method:             "GET",
		PathPattern:        "/services/{id}/service-edge-router-policies",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceServiceEdgeRouterPoliciesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceServiceEdgeRouterPoliciesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceServiceEdgeRouterPolicies: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceServicePolicies lists service policies that affect a specific service

  Retrieves a list of service policy resources that affect specific service; supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServiceServicePolicies(params *ListServiceServicePoliciesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceServicePoliciesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceServicePoliciesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceServicePolicies",
		Method:             "GET",
		PathPattern:        "/services/{id}/service-policies",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceServicePoliciesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceServicePoliciesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceServicePolicies: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServiceTerminators lists of terminators assigned to a service

  Retrieves a list of terminator resources that are assigned specific service; supports filtering, sorting, and pagination.

*/
func (a *Client) ListServiceTerminators(params *ListServiceTerminatorsParams, authInfo runtime.ClientAuthInfoWriter) (*ListServiceTerminatorsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServiceTerminatorsParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServiceTerminators",
		Method:             "GET",
		PathPattern:        "/services/{id}/terminators",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServiceTerminatorsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServiceTerminatorsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServiceTerminators: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  ListServices lists services

  Retrieves a list of config resources; supports filtering, sorting, and pagination. Requires admin access.

*/
func (a *Client) ListServices(params *ListServicesParams, authInfo runtime.ClientAuthInfoWriter) (*ListServicesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListServicesParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "listServices",
		Method:             "GET",
		PathPattern:        "/services",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &ListServicesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListServicesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for listServices: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  PatchService updates the supplied fields on a service

  Update the supplied fields on a service. Requires admin access.
*/
func (a *Client) PatchService(params *PatchServiceParams, authInfo runtime.ClientAuthInfoWriter) (*PatchServiceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchServiceParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "patchService",
		Method:             "PATCH",
		PathPattern:        "/services/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &PatchServiceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchServiceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for patchService: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

/*
  UpdateService updates all fields on a service

  Update all fields on a service by id. Requires admin access.
*/
func (a *Client) UpdateService(params *UpdateServiceParams, authInfo runtime.ClientAuthInfoWriter) (*UpdateServiceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUpdateServiceParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "updateService",
		Method:             "PUT",
		PathPattern:        "/services/{id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params:             params,
		Reader:             &UpdateServiceReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	success, ok := result.(*UpdateServiceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	// safeguard: normally, absent a default response, unknown success responses return an error above: so this is a codegen issue
	msg := fmt.Sprintf("unexpected success response for updateService: API contract not enforced by server. Client expected to get an error, but got: %T", result)
	panic(msg)
}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}
