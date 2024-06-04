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

package handler_mgmt

import (
	"github.com/openziti/channel/v2"
	"github.com/openziti/foundation/v2/concurrenz"
	"github.com/openziti/ziti/common/trace"
	"github.com/openziti/ziti/controller/network"
	"github.com/openziti/ziti/controller/xmgmt"
)

type BindHandler struct {
	network *network.Network
	xmgmts  *concurrenz.CopyOnWriteSlice[xmgmt.Xmgmt]
}

func NewBindHandler(network *network.Network, xmgmts *concurrenz.CopyOnWriteSlice[xmgmt.Xmgmt]) channel.BindHandler {
	return &BindHandler{network: network, xmgmts: xmgmts}
}

func (bindHandler *BindHandler) BindChannel(binding channel.Binding) error {
	inspectRequestHandler := newInspectHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(&channel.AsyncFunctionReceiveAdapter{
		Type:    inspectRequestHandler.ContentType(),
		Handler: inspectRequestHandler.HandleReceive,
	})

	validateTerminatorsRequestHandler := newValidateTerminatorsHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(&channel.AsyncFunctionReceiveAdapter{
		Type:    validateTerminatorsRequestHandler.ContentType(),
		Handler: validateTerminatorsRequestHandler.HandleReceive,
	})

	validateLinksRequestHandler := newValidateRouterLinksHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(&channel.AsyncFunctionReceiveAdapter{
		Type:    validateLinksRequestHandler.ContentType(),
		Handler: validateLinksRequestHandler.HandleReceive,
	})

	validateSdkTerminatorsRequestHandler := newValidateRouterSdkTerminatorsHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(&channel.AsyncFunctionReceiveAdapter{
		Type:    validateSdkTerminatorsRequestHandler.ContentType(),
		Handler: validateSdkTerminatorsRequestHandler.HandleReceive,
	})

	tracesHandler := newStreamTracesHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(tracesHandler)
	binding.AddCloseHandler(tracesHandler)

	eventsHandler := newStreamEventsHandler(bindHandler.network)
	binding.AddTypedReceiveHandler(eventsHandler)
	binding.AddCloseHandler(eventsHandler)

	binding.AddTypedReceiveHandler(newTogglePipeTracesHandler(bindHandler.network))

	binding.AddPeekHandler(trace.NewChannelPeekHandler(bindHandler.network.GetAppId(), binding.GetChannel(), bindHandler.network.GetTraceController()))

	xmgmtDone := make(chan struct{})
	for _, x := range bindHandler.xmgmts.Value() {
		if err := binding.Bind(x); err != nil {
			return err
		}
		if err := x.Run(binding.GetChannel(), xmgmtDone); err != nil {
			return err
		}
	}
	if len(bindHandler.xmgmts.Value()) > 0 {
		binding.AddCloseHandler(newXmgmtCloseHandler(xmgmtDone))
	}

	return nil
}
