// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package wickr

import (
	"context"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wickr"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/awsv2"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/framework"
)

// Per design.md → "Sweepers", the Wickr package ships exactly one sweeper:
// `aws_wickr_network`. Child-object sweepers are unnecessary because
// DeleteNetwork is documented as cascading through all child resources
// (users, bots, security groups, settings).
func RegisterSweepers() {
	awsv2.Register("aws_wickr_network", sweepNetworks)
}

func sweepNetworks(ctx context.Context, client *conns.AWSClient) ([]sweep.Sweepable, error) {
	conn := client.WickrClient(ctx)
	var sweepResources []sweep.Sweepable

	input := wickr.ListNetworksInput{}
	pages := wickr.NewListNetworksPaginator(conn, &input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, n := range page.Networks {
			name := aws.ToString(n.NetworkName)
			// Sweep only networks whose names start with the acceptance-test
			// prefix convention so human-created networks are left alone.
			if !strings.HasPrefix(name, sweep.ResourcePrefix) {
				log.Printf("[INFO] Skipping Wickr Network %s", name)
				continue
			}
			sweepResources = append(sweepResources, framework.NewSweepResource(newNetworkResource, client,
				framework.NewAttribute("network_id", aws.ToString(n.NetworkId)),
			))
		}
	}

	return sweepResources, nil
}
