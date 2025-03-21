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

package exporter

import (
	"errors"
	"github.com/openziti/edge-api/rest_management_api_client/auth_policy"
	"github.com/openziti/edge-api/rest_management_api_client/identity"
	"github.com/openziti/edge-api/rest_model"
	"github.com/openziti/ziti/internal/ascode"
	"slices"
)

func (exporter Exporter) IsIdentityExportRequired(args []string) bool {
	return slices.Contains(args, "all") || len(args) == 0 || // explicit all or nothing specified
		slices.Contains(args, "identity")
}

func (exporter Exporter) GetIdentities() ([]map[string]interface{}, error) {

	return exporter.getEntities(
		"Identities",

		func() (int64, error) {
			limit := int64(1)
			resp, err := exporter.Client.Identity.ListIdentities(&identity.ListIdentitiesParams{Limit: &limit}, nil)
			if err != nil {
				return -1, err
			}
			return *resp.GetPayload().Meta.Pagination.TotalCount, nil
		},

		func(offset *int64, limit *int64) ([]interface{}, error) {
			resp, err := exporter.Client.Identity.ListIdentities(&identity.ListIdentitiesParams{Offset: offset, Limit: limit}, nil)
			if err != nil {
				return nil, err
			}
			entities := make([]interface{}, len(resp.GetPayload().Data))
			for i, c := range resp.GetPayload().Data {
				entities[i] = interface{}(c)
			}
			return entities, nil
		},

		func(entity interface{}) (map[string]interface{}, error) {

			item := entity.(*rest_model.IdentityDetail)

			// convert to a map of values
			m, err := exporter.ToMap(item)
			if err != nil {
				log.WithError(err).Error("error converting Identity to map")
			}
			exporter.defaultRoleAttributes(m)

			// filter unwanted properties
			exporter.Filter(m, []string{"id", "_links", "createdAt", "updatedAt",
				"defaultHostingCost", "defaultHostingPrecedence", "hasApiSession", "serviceHostingPrecedences", "enrollment",
				"appData", "sdkInfo", "disabledAt", "disabledUntil", "serviceHostingCosts", "envInfo", "authenticators", "type", "authPolicyId",
				"hasRouterConnection", "hasEdgeRouterConnection"})

			if item.DisabledUntil != nil {
				m["disabledUntil"] = item.DisabledUntil
			}

			// translate ids to names
			authPolicy, lookupErr := ascode.GetItemFromCache(exporter.authPolicyCache, *item.AuthPolicyID, func(id string) (interface{}, error) {
				return exporter.Client.AuthPolicy.DetailAuthPolicy(&auth_policy.DetailAuthPolicyParams{ID: id}, nil)
			})
			if lookupErr != nil {
				return nil, errors.Join(errors.New("error reading Auth Policy: "+*item.AuthPolicyID), lookupErr)
			}
			m["authPolicy"] = "@" + *authPolicy.(*auth_policy.DetailAuthPolicyOK).GetPayload().Data.Name
			return m, nil
		})

}
