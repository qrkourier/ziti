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
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/openziti/edge/rest_model"
)

// UpdateExternalJwtSignerOKCode is the HTTP code returned for type UpdateExternalJwtSignerOK
const UpdateExternalJwtSignerOKCode int = 200

/*UpdateExternalJwtSignerOK The update request was successful and the resource has been altered

swagger:response updateExternalJwtSignerOK
*/
type UpdateExternalJwtSignerOK struct {

	/*
	  In: Body
	*/
	Payload *rest_model.Empty `json:"body,omitempty"`
}

// NewUpdateExternalJwtSignerOK creates UpdateExternalJwtSignerOK with default headers values
func NewUpdateExternalJwtSignerOK() *UpdateExternalJwtSignerOK {

	return &UpdateExternalJwtSignerOK{}
}

// WithPayload adds the payload to the update external jwt signer o k response
func (o *UpdateExternalJwtSignerOK) WithPayload(payload *rest_model.Empty) *UpdateExternalJwtSignerOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the update external jwt signer o k response
func (o *UpdateExternalJwtSignerOK) SetPayload(payload *rest_model.Empty) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UpdateExternalJwtSignerOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// UpdateExternalJwtSignerBadRequestCode is the HTTP code returned for type UpdateExternalJwtSignerBadRequest
const UpdateExternalJwtSignerBadRequestCode int = 400

/*UpdateExternalJwtSignerBadRequest The supplied request contains invalid fields or could not be parsed (json and non-json bodies). The error's code, message, and cause fields can be inspected for further information

swagger:response updateExternalJwtSignerBadRequest
*/
type UpdateExternalJwtSignerBadRequest struct {

	/*
	  In: Body
	*/
	Payload *rest_model.APIErrorEnvelope `json:"body,omitempty"`
}

// NewUpdateExternalJwtSignerBadRequest creates UpdateExternalJwtSignerBadRequest with default headers values
func NewUpdateExternalJwtSignerBadRequest() *UpdateExternalJwtSignerBadRequest {

	return &UpdateExternalJwtSignerBadRequest{}
}

// WithPayload adds the payload to the update external jwt signer bad request response
func (o *UpdateExternalJwtSignerBadRequest) WithPayload(payload *rest_model.APIErrorEnvelope) *UpdateExternalJwtSignerBadRequest {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the update external jwt signer bad request response
func (o *UpdateExternalJwtSignerBadRequest) SetPayload(payload *rest_model.APIErrorEnvelope) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UpdateExternalJwtSignerBadRequest) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(400)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// UpdateExternalJwtSignerUnauthorizedCode is the HTTP code returned for type UpdateExternalJwtSignerUnauthorized
const UpdateExternalJwtSignerUnauthorizedCode int = 401

/*UpdateExternalJwtSignerUnauthorized The currently supplied session does not have the correct access rights to request this resource

swagger:response updateExternalJwtSignerUnauthorized
*/
type UpdateExternalJwtSignerUnauthorized struct {

	/*
	  In: Body
	*/
	Payload *rest_model.APIErrorEnvelope `json:"body,omitempty"`
}

// NewUpdateExternalJwtSignerUnauthorized creates UpdateExternalJwtSignerUnauthorized with default headers values
func NewUpdateExternalJwtSignerUnauthorized() *UpdateExternalJwtSignerUnauthorized {

	return &UpdateExternalJwtSignerUnauthorized{}
}

// WithPayload adds the payload to the update external jwt signer unauthorized response
func (o *UpdateExternalJwtSignerUnauthorized) WithPayload(payload *rest_model.APIErrorEnvelope) *UpdateExternalJwtSignerUnauthorized {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the update external jwt signer unauthorized response
func (o *UpdateExternalJwtSignerUnauthorized) SetPayload(payload *rest_model.APIErrorEnvelope) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UpdateExternalJwtSignerUnauthorized) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(401)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// UpdateExternalJwtSignerNotFoundCode is the HTTP code returned for type UpdateExternalJwtSignerNotFound
const UpdateExternalJwtSignerNotFoundCode int = 404

/*UpdateExternalJwtSignerNotFound The requested resource does not exist

swagger:response updateExternalJwtSignerNotFound
*/
type UpdateExternalJwtSignerNotFound struct {

	/*
	  In: Body
	*/
	Payload *rest_model.APIErrorEnvelope `json:"body,omitempty"`
}

// NewUpdateExternalJwtSignerNotFound creates UpdateExternalJwtSignerNotFound with default headers values
func NewUpdateExternalJwtSignerNotFound() *UpdateExternalJwtSignerNotFound {

	return &UpdateExternalJwtSignerNotFound{}
}

// WithPayload adds the payload to the update external jwt signer not found response
func (o *UpdateExternalJwtSignerNotFound) WithPayload(payload *rest_model.APIErrorEnvelope) *UpdateExternalJwtSignerNotFound {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the update external jwt signer not found response
func (o *UpdateExternalJwtSignerNotFound) SetPayload(payload *rest_model.APIErrorEnvelope) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UpdateExternalJwtSignerNotFound) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(404)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
