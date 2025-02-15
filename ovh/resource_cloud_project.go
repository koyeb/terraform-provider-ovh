package ovh

import (
	"fmt"
	"log"
	"net/url"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/ovh/go-ovh/ovh"
	"github.com/ovh/terraform-provider-ovh/ovh/helpers"
)

var (
	publicCloudProjectNameFormatRegex = regexp.MustCompile("^[0-9a-f]{12}4[0-9a-f]{19}$")
)

func resourceCloudProject() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudProjectCreate,
		Update: resourceCloudProjectUpdate,
		Read:   resourceCloudProjectRead,
		Delete: resourceCloudProjectDelete,

		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: resourceCloudProjectSchema(),
	}
}

func resourceCloudProjectSchema() map[string]*schema.Schema {
	schema := map[string]*schema.Schema{
		"description": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},

		// computed
		"urn": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"project_name": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"project_id": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"access": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"status": {
			Type:     schema.TypeString,
			Computed: true,
		},
	}

	for k, v := range genericOrderSchema(false) {
		schema[k] = v
	}

	return schema
}

func resourceCloudProjectCreate(d *schema.ResourceData, meta interface{}) error {
	if err := orderCreate(d, meta, "cloud"); err != nil {
		return fmt.Errorf("Could not order cloud project: %q", err)
	}

	return resourceCloudProjectUpdate(d, meta)
}

func resourceCloudProjectUpdate(d *schema.ResourceData, meta interface{}) error {
	order, details, err := orderRead(d, meta)
	if err != nil {
		return fmt.Errorf("Could not read cloud project order: %q", err)
	}

	config := meta.(*Config)
	serviceName := details[0].Domain

	// in the US, for reasons too long to be detailled here, cloud project order domain is not the public cloud project id, but "*".
	// There have been discussions to align US & EU, but they've failed.
	// So we end up making a few extra queries to fetch the project id from operations details.
	if !publicCloudProjectNameFormatRegex.MatchString(serviceName) {
		orderDetailId := details[0].OrderDetailId
		operations, err := orderDetailOperations(config.OVHClient, order.OrderId, orderDetailId)
		if err != nil {
			return fmt.Errorf("Could not read cloudProject order details operations: %q", err)
		}
		for _, operation := range operations {
			if !publicCloudProjectNameFormatRegex.MatchString(operation.Resource.Name) {
				continue
			}
			serviceName = operation.Resource.Name
		}
	}

	log.Printf("[DEBUG] Will update cloudProject: %s", serviceName)
	opts := (&CloudProjectUpdateOpts{}).FromResource(d)
	endpoint := fmt.Sprintf("/cloud/project/%s", url.PathEscape(serviceName))
	if err := config.OVHClient.Put(endpoint, opts, nil); err != nil {
		return fmt.Errorf("calling Put %s: %q", endpoint, err)
	}

	return resourceCloudProjectRead(d, meta)
}

func resourceCloudProjectRead(d *schema.ResourceData, meta interface{}) error {
	order, details, err := orderRead(d, meta)
	if err != nil {
		return fmt.Errorf("Could not read cloudProject order: %q", err)
	}

	config := meta.(*Config)
	serviceName := details[0].Domain

	// in the US, for reasons too long to be detailled here, cloud project order domain is not the public cloud project id, but "*".
	// There have been discussions to align US & EU, but they've failed.
	// So we end up making a few extra queries to fetch the project id from operations details.
	if !publicCloudProjectNameFormatRegex.MatchString(serviceName) {
		orderDetailId := details[0].OrderDetailId
		operations, err := orderDetailOperations(config.OVHClient, order.OrderId, orderDetailId)
		if err != nil {
			return fmt.Errorf("Could not read cloudProject order details operations: %q", err)
		}
		for _, operation := range operations {
			if !publicCloudProjectNameFormatRegex.MatchString(operation.Resource.Name) {
				continue
			}
			serviceName = operation.Resource.Name
		}
	}

	log.Printf("[DEBUG] Will read cloudProject: %s", serviceName)
	r := &CloudProject{}
	endpoint := fmt.Sprintf("/cloud/project/%s", url.PathEscape(serviceName))
	if err := config.OVHClient.Get(endpoint, &r); err != nil {
		return helpers.CheckDeleted(d, err, endpoint)
	}

	// set resource attributes
	for k, v := range r.ToMap() {
		d.Set(k, v)
	}

	return nil
}

func resourceCloudProjectDelete(d *schema.ResourceData, meta interface{}) error {
	order, details, err := orderRead(d, meta)
	if err != nil {
		return fmt.Errorf("Could not read cloudProject order: %q", err)
	}

	config := meta.(*Config)
	serviceName := details[0].Domain

	// in the US, for reasons too long to be detailled here, cloud project order domain is not the public cloud project id, but "*".
	// There have been discussions to align US & EU, but they've failed.
	// So we end up making a few extra queries to fetch the project id from operations details.
	if !publicCloudProjectNameFormatRegex.MatchString(serviceName) {
		orderDetailId := details[0].OrderDetailId
		operations, err := orderDetailOperations(config.OVHClient, order.OrderId, orderDetailId)
		if err != nil {
			return fmt.Errorf("Could not read cloudProject order details operations: %q", err)
		}
		for _, operation := range operations {
			if !publicCloudProjectNameFormatRegex.MatchString(operation.Resource.Name) {
				continue
			}
			serviceName = operation.Resource.Name
		}
	}

	id := d.Id()

	terminate := func() (string, error) {
		log.Printf("[DEBUG] Will terminate cloud project %s for order %s", serviceName, id)
		endpoint := fmt.Sprintf(
			"/cloud/project/%s/terminate",
			url.PathEscape(serviceName),
		)
		if err := config.OVHClient.Post(endpoint, nil, nil); err != nil {
			if errOvh, ok := err.(*ovh.APIError); ok && (errOvh.Code == 404 || errOvh.Code == 460) {
				return "", nil
			}
			return "", fmt.Errorf("calling Post %s:\n\t %q", endpoint, err)
		}
		return serviceName, nil
	}

	confirmTerminate := func(token string) error {
		log.Printf("[DEBUG] Will confirm termination of cloud project %s for order %s", serviceName, id)
		endpoint := fmt.Sprintf(
			"/cloud/project/%s/confirmTermination",
			url.PathEscape(serviceName),
		)
		if err := config.OVHClient.Post(endpoint, &CloudProjectConfirmTerminationOpts{Token: token}, nil); err != nil {
			return fmt.Errorf("calling Post %s:\n\t %q", endpoint, err)
		}
		return nil
	}

	if err := orderDelete(d, meta, terminate, confirmTerminate); err != nil {
		return err
	}

	d.SetId("")
	return nil
}
