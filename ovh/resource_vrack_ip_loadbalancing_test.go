package ovh

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/ovh/go-ovh/ovh"
)

var testAccVrackIpLoadbalancingConfig = fmt.Sprintf(`
data "ovh_iploadbalancing" "iplb" {
  service_name = "%s"
}

resource "ovh_vrack_iploadbalancing" "viplb" {
  service_name     = "%s"
  ip_loadbalancing = data.ovh_iploadbalancing.iplb.service_name
}
`, os.Getenv("OVH_IPLB_SERVICE_TEST"), os.Getenv("OVH_VRACK_SERVICE_TEST"))

func init() {
	resource.AddTestSweepers("ovh_vrack_iploadbalancing", &resource.Sweeper{
		Name:         "ovh_vrack_iploadbalancing",
		Dependencies: []string{"ovh_iploadbalancing_vrack_network"},
		F:            testSweepVrackIpLoadbalancing,
	})
}

func testSweepVrackIpLoadbalancing(region string) error {
	client, err := sharedClientForRegion(region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}

	serviceName := os.Getenv("OVH_VRACK_SERVICE_TEST")
	if serviceName == "" {
		log.Print("[DEBUG] OVH_VRACK_SERVICE_TEST is not set. Unable to sweep VrackIpLoadbalancing")
		return nil
	}
	ipLoadbalancing := os.Getenv("OVH_IPLB_SERVICE_TEST")
	if ipLoadbalancing == "" {
		log.Print("[DEBUG] OVH_IPLB_SERVICE_TEST is not set. Unable to sweep VrackIpLoadbalancing")
		return nil
	}
	endpoint := fmt.Sprintf("/vrack/%s/ipLoadbalancing/%s",
		url.PathEscape(serviceName),
		url.PathEscape(ipLoadbalancing),
	)

	viplb := &VrackIpLoadbalancing{}

	if err := client.Get(endpoint, viplb); err != nil {
		if errOvh, ok := err.(*ovh.APIError); ok && errOvh.Code == 404 {
			return nil
		}
		return err
	}

	task := &VrackTask{}

	if err := client.Delete(endpoint, task); err != nil {
		return fmt.Errorf("Error calling DELETE %s with %s/%s:\n\t %q", endpoint, serviceName, ipLoadbalancing, err)
	}

	if err := waitForVrackTask(task, client); err != nil {
		return fmt.Errorf("Error waiting for vrack (%s) to detach cloud project (%s): %s", serviceName, ipLoadbalancing, err)
	}

	return nil
}

func TestAccVrackIpLoadbalancing_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccCheckVrackIpLoadbalancingPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVrackIpLoadbalancingConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("ovh_vrack_iploadbalancing.viplb", "service_name", os.Getenv("OVH_VRACK_SERVICE_TEST")),
					resource.TestCheckResourceAttr("ovh_vrack_iploadbalancing.viplb", "ip_loadbalancing", os.Getenv("OVH_IPLB_SERVICE_TEST")),
				),
			},
		},
	})
}

func testAccCheckVrackIpLoadbalancingPreCheck(t *testing.T) {
	testAccPreCheckVRack(t)
	testAccCheckVRackExists(t)
	testAccPreCheckIpLoadbalancing(t)
}
