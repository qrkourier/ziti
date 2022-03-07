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

package external_j_w_t_signer

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

// DeleteExternalJwtSignerHandlerFunc turns a function with the right signature into a delete external jwt signer handler
type DeleteExternalJwtSignerHandlerFunc func(DeleteExternalJwtSignerParams, interface{}) middleware.Responder

// Handle executing the request and returning a response
func (fn DeleteExternalJwtSignerHandlerFunc) Handle(params DeleteExternalJwtSignerParams, principal interface{}) middleware.Responder {
	return fn(params, principal)
}

// DeleteExternalJwtSignerHandler interface for that can handle valid delete external jwt signer params
type DeleteExternalJwtSignerHandler interface {
	Handle(DeleteExternalJwtSignerParams, interface{}) middleware.Responder
}

// NewDeleteExternalJwtSigner creates a new http.Handler for the delete external jwt signer operation
func NewDeleteExternalJwtSigner(ctx *middleware.Context, handler DeleteExternalJwtSignerHandler) *DeleteExternalJwtSigner {
	return &DeleteExternalJwtSigner{Context: ctx, Handler: handler}
}

/* DeleteExternalJwtSigner swagger:route DELETE /external-jwt-signers/{id} External JWT Signer deleteExternalJwtSigner

Delete an External JWT Signer

Delete an External JWT Signer by id. Requires admin access.


*/
type DeleteExternalJwtSigner struct {
	Context *middleware.Context
	Handler DeleteExternalJwtSignerHandler
}

func (o *DeleteExternalJwtSigner) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, rCtx, _ := o.Context.RouteInfo(r)
	if rCtx != nil {
		*r = *rCtx
	}
	var Params = NewDeleteExternalJwtSignerParams()
	uprinc, aCtx, err := o.Context.Authorize(r, route)
	if err != nil {
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}
	if aCtx != nil {
		*r = *aCtx
	}
	var principal interface{}
	if uprinc != nil {
		principal = uprinc.(interface{}) // this is really a interface{}, I promise
	}

	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params, principal) // actually handle the request
	o.Context.Respond(rw, r, route.Produces, route, res)

}
