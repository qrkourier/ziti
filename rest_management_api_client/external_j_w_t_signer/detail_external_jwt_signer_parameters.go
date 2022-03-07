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
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewDetailExternalJwtSignerParams creates a new DetailExternalJwtSignerParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDetailExternalJwtSignerParams() *DetailExternalJwtSignerParams {
	return &DetailExternalJwtSignerParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDetailExternalJwtSignerParamsWithTimeout creates a new DetailExternalJwtSignerParams object
// with the ability to set a timeout on a request.
func NewDetailExternalJwtSignerParamsWithTimeout(timeout time.Duration) *DetailExternalJwtSignerParams {
	return &DetailExternalJwtSignerParams{
		timeout: timeout,
	}
}

// NewDetailExternalJwtSignerParamsWithContext creates a new DetailExternalJwtSignerParams object
// with the ability to set a context for a request.
func NewDetailExternalJwtSignerParamsWithContext(ctx context.Context) *DetailExternalJwtSignerParams {
	return &DetailExternalJwtSignerParams{
		Context: ctx,
	}
}

// NewDetailExternalJwtSignerParamsWithHTTPClient creates a new DetailExternalJwtSignerParams object
// with the ability to set a custom HTTPClient for a request.
func NewDetailExternalJwtSignerParamsWithHTTPClient(client *http.Client) *DetailExternalJwtSignerParams {
	return &DetailExternalJwtSignerParams{
		HTTPClient: client,
	}
}

/* DetailExternalJwtSignerParams contains all the parameters to send to the API endpoint
   for the detail external jwt signer operation.

   Typically these are written to a http.Request.
*/
type DetailExternalJwtSignerParams struct {

	/* ID.

	   The id of the requested resource
	*/
	ID string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the detail external jwt signer params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DetailExternalJwtSignerParams) WithDefaults() *DetailExternalJwtSignerParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the detail external jwt signer params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DetailExternalJwtSignerParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) WithTimeout(timeout time.Duration) *DetailExternalJwtSignerParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) WithContext(ctx context.Context) *DetailExternalJwtSignerParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) WithHTTPClient(client *http.Client) *DetailExternalJwtSignerParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithID adds the id to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) WithID(id string) *DetailExternalJwtSignerParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the detail external jwt signer params
func (o *DetailExternalJwtSignerParams) SetID(id string) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *DetailExternalJwtSignerParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
