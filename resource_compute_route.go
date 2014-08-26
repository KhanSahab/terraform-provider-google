package google

import (
	"fmt"
	"log"
	"time"

	"code.google.com/p/google-api-go-client/compute/v1"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceComputeRoute() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeRouteCreate,
		Read:   resourceComputeRouteRead,
		Delete: resourceComputeRouteDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"dest_range": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"network": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"next_hop_ip": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"next_hop_instance": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"next_hop_instance_zone": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"next_hop_gateway": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"next_hop_network": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"priority": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},

			"tags": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set: func(v interface{}) int {
					return hashcode.String(v.(string))
				},
			},
		},
	}
}

func resourceComputeRouteCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	// Look up the network to attach the route to
	network, err := config.clientCompute.Networks.Get(
		config.Project, d.Get("network").(string)).Do()
	if err != nil {
		return fmt.Errorf("Error reading network: %s", err)
	}

	// Next hop data
	var nextHopInstance, nextHopIp, nextHopNetwork, nextHopGateway string
	if v, ok := d.GetOk("next_hop_ip"); ok {
		nextHopIp = v.(string)
	}
	if v, ok := d.GetOk("next_hop_gateway"); ok {
		nextHopGateway = v.(string)
	}
	if v, ok := d.GetOk("next_hop_instance"); ok {
		nextInstance, err := config.clientCompute.Instances.Get(
			config.Project,
			d.Get("next_hop_instance_zone").(string),
			v.(string)).Do()
		if err != nil {
			return fmt.Errorf("Error reading instance: %s", err)
		}

		nextHopInstance = nextInstance.SelfLink
	}
	if v, ok := d.GetOk("next_hop_network"); ok {
		nextNetwork, err := config.clientCompute.Networks.Get(
			config.Project, v.(string)).Do()
		if err != nil {
			return fmt.Errorf("Error reading network: %s", err)
		}

		nextHopNetwork = nextNetwork.SelfLink
	}

	// Tags
	var tags []string
	if v := d.Get("tags").(*schema.Set); v.Len() > 0 {
		tags = make([]string, v.Len())
		for i, v := range v.List() {
			tags[i] = v.(string)
		}
	}

	// Build the route parameter
	route := &compute.Route{
		Name:            d.Get("name").(string),
		DestRange:       d.Get("dest_range").(string),
		Network:         network.SelfLink,
		NextHopInstance: nextHopInstance,
		NextHopIp:       nextHopIp,
		NextHopNetwork:  nextHopNetwork,
		NextHopGateway:  nextHopGateway,
		Priority:        int64(d.Get("priority").(int)),
		Tags:            tags,
	}
	log.Printf("[DEBUG] Route insert request: %#v", route)
	op, err := config.clientCompute.Routes.Insert(
		config.Project, route).Do()
	if err != nil {
		return fmt.Errorf("Error creating route: %s", err)
	}

	// It probably maybe worked, so store the ID now
	d.SetId(route.Name)

	// Wait for the operation to complete
	w := &OperationWaiter{
		Service: config.clientCompute,
		Op:      op,
		Project: config.Project,
		Type:    OperationWaitGlobal,
	}
	state := w.Conf()
	state.Timeout = 2 * time.Minute
	state.MinTimeout = 1 * time.Second
	opRaw, err := state.WaitForState()
	if err != nil {
		return fmt.Errorf("Error waiting for route to create: %s", err)
	}
	op = opRaw.(*compute.Operation)
	if op.Error != nil {
		// The resource didn't actually create
		d.SetId("")

		// Return the error
		return OperationError(*op.Error)
	}

	return resourceComputeRouteRead(d, meta)
}

func resourceComputeRouteRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	_, err := config.clientCompute.Routes.Get(
		config.Project, d.Id()).Do()
	if err != nil {
		return fmt.Errorf("Error reading route: %#v", err)
	}

	return nil
}

func resourceComputeRouteDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	// Delete the route
	op, err := config.clientCompute.Routes.Delete(
		config.Project, d.Id()).Do()
	if err != nil {
		return fmt.Errorf("Error deleting route: %s", err)
	}

	// Wait for the operation to complete
	w := &OperationWaiter{
		Service: config.clientCompute,
		Op:      op,
		Project: config.Project,
		Type:    OperationWaitGlobal,
	}
	state := w.Conf()
	state.Timeout = 2 * time.Minute
	state.MinTimeout = 1 * time.Second
	opRaw, err := state.WaitForState()
	if err != nil {
		return fmt.Errorf("Error waiting for route to delete: %s", err)
	}
	op = opRaw.(*compute.Operation)
	if op.Error != nil {
		// Return the error
		return OperationError(*op.Error)
	}

	d.SetId("")
	return nil
}
