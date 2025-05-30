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

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/channel/v4"
	"github.com/openziti/channel/v4/protobufs"
	"github.com/openziti/fablab/kernel/model"
	"github.com/openziti/ziti/common/pb/mgmt_pb"
	"github.com/openziti/ziti/controller/rest_client/terminator"
	"github.com/openziti/ziti/zitirest"
	"github.com/openziti/ziti/zititest/zitilab/chaos"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"time"
)

// start with a random scenario then cycle through them
var scenarioCounter = rand.Intn(7)

func sowChaos(run model.Run) error {
	var controllers []*model.Component
	var err error

	scenarioCounter = (scenarioCounter + 1) % 3
	scenario := scenarioCounter + 1

	if scenario&0b001 > 0 {
		controllers, err = chaos.SelectRandom(run, ".ctrl", chaos.RandomOfTotal())
		if err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
	}

	var routers []*model.Component
	if scenario&0b010 > 0 {
		routers, err = chaos.SelectRandom(run, ".router", chaos.PercentageRange(10, 75))
		if err != nil {
			return err
		}
	}

	toRestart := append(([]*model.Component)(nil), controllers...)
	toRestart = append(toRestart, routers...)
	fmt.Printf("restarting %d controllers and %d routers\n", len(controllers), len(routers))
	return chaos.RestartSelected(run, 100, toRestart...)
}

func validateTerminators(run model.Run) error {
	ctrls := run.GetModel().SelectComponents(".ctrl")
	errC := make(chan error, len(ctrls))
	deadline := time.Now().Add(15 * time.Minute)
	for _, ctrl := range ctrls {
		ctrlComponent := ctrl
		go validateTerminatorsForCtrlWithChan(ctrlComponent, deadline, errC)
	}

	for i := 0; i < len(ctrls); i++ {
		err := <-errC
		if err != nil {
			return err
		}
	}

	return nil
}

func validateTerminatorsForCtrlWithChan(c *model.Component, deadline time.Time, errC chan<- error) {
	errC <- validateTerminatorsForCtrl(c, deadline)
}

func validateTerminatorsForCtrl(c *model.Component, deadline time.Time) error {
	expectedTerminatorCount := len(c.GetModel().SelectComponents(".router")) * 10
	username := c.MustStringVariable("credentials.edge.username")
	password := c.MustStringVariable("credentials.edge.password")
	edgeApiBaseUrl := c.Host.PublicIp + ":1280"

	var clients *zitirest.Clients
	loginStart := time.Now()
	for {
		var err error
		clients, err = zitirest.NewManagementClients(edgeApiBaseUrl)
		if err != nil {
			if time.Since(loginStart) > time.Minute {
				return err
			}
			pfxlog.Logger().WithField("ctrlId", c.Id).WithError(err).Info("failed to initialize mgmt client, trying again in 1s")
			time.Sleep(time.Second)
			continue
		}

		if err = clients.Authenticate(username, password); err != nil {
			if time.Since(loginStart) > time.Minute {
				return err
			}
			pfxlog.Logger().WithField("ctrlId", c.Id).WithError(err).Info("failed to authenticate, trying again in 1s")
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	terminatorsPresent := false
	start := time.Now()

	logger := pfxlog.Logger().WithField("ctrl", c.Id)
	var lastLog time.Time
	for time.Now().Before(deadline) && !terminatorsPresent {
		terminatorCount, err := getTerminatorCount(clients)
		if err != nil {
			return nil
		}
		if terminatorCount == int64(expectedTerminatorCount) {
			terminatorsPresent = true
		} else {
			time.Sleep(5 * time.Second)
		}
		if time.Since(lastLog) > time.Minute {
			logger.Infof("current terminator count: %v, elapsed time: %v", terminatorCount, time.Since(start))
			lastLog = time.Now()
		}
	}

	if terminatorsPresent {
		logger.Infof("all terminators present, elapsed time: %v", time.Since(start))
	} else {
		return fmt.Errorf("fail to reach expected terminator count of %v on controller %v", expectedTerminatorCount, c.Id)
	}

	for {
		count, err := validateRouterErtTerminators(c.Id, clients)
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}

		logger.Infof("current count of invalid ert terminators: %v, elapsed time: %v", count, time.Since(start))
		time.Sleep(15 * time.Second)
	}
}

func getTerminatorCount(clients *zitirest.Clients) (int64, error) {
	ctx, cancelF := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelF()

	filter := "limit 1"
	result, err := clients.Fabric.Terminator.ListTerminators(&terminator.ListTerminatorsParams{
		Filter:  &filter,
		Context: ctx,
	})

	if err != nil {
		return 0, err
	}
	count := *result.Payload.Meta.Pagination.TotalCount
	return count, nil
}

func validateRouterErtTerminators(id string, clients *zitirest.Clients) (int, error) {
	logger := pfxlog.Logger().WithField("ctrl", id)

	closeNotify := make(chan struct{})
	eventNotify := make(chan *mgmt_pb.RouterErtTerminatorsDetails, 1)

	handleErtTerminatorResults := func(msg *channel.Message, _ channel.Channel) {
		detail := &mgmt_pb.RouterErtTerminatorsDetails{}
		if err := proto.Unmarshal(msg.Body, detail); err != nil {
			pfxlog.Logger().WithError(err).Error("unable to unmarshal router ert terminator details")
			return
		}
		eventNotify <- detail
	}

	bindHandler := func(binding channel.Binding) error {
		binding.AddReceiveHandlerF(int32(mgmt_pb.ContentType_ValidateRouterErtTerminatorsResultType), handleErtTerminatorResults)
		binding.AddCloseHandler(channel.CloseHandlerF(func(ch channel.Channel) {
			close(closeNotify)
		}))
		return nil
	}

	ch, err := clients.NewWsMgmtChannel(channel.BindHandlerF(bindHandler))
	if err != nil {
		return 0, err
	}

	defer func() {
		_ = ch.Close()
	}()

	request := &mgmt_pb.ValidateRouterErtTerminatorsRequest{
		Filter: "limit none",
	}
	responseMsg, err := protobufs.MarshalTyped(request).WithTimeout(10 * time.Second).SendForReply(ch)

	response := &mgmt_pb.ValidateRouterErtTerminatorsResponse{}
	if err = protobufs.TypedResponse(response).Unmarshall(responseMsg, err); err != nil {
		return 0, err
	}

	if !response.Success {
		return 0, fmt.Errorf("failed to start ert terminator validation: %s", response.Message)
	}

	logger.Infof("started validation of %v routers", response.RouterCount)

	expected := response.RouterCount

	invalid := 0
	for expected > 0 {
		select {
		case <-closeNotify:
			fmt.Printf("channel closed, exiting")
			return 0, errors.New("unexpected close of mgmt channel")
		case routerDetail := <-eventNotify:
			if !routerDetail.ValidateSuccess {
				return invalid, fmt.Errorf("error: unable to validate on controller %s (%s)", routerDetail.Message, id)
			}
			for _, linkDetail := range routerDetail.Details {
				if !linkDetail.IsValid {
					invalid++
				}
			}
			expected--
		}
	}
	if invalid == 0 {
		logger.Infof("ert terminator validation of %v routers successful", response.RouterCount)
		return invalid, nil
	}
	return invalid, fmt.Errorf("invalid ert terminators found")
}
