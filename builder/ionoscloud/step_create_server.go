// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ionoscloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	ionoscloud "github.com/ionos-cloud/sdk-go/v6"
)

type stepCreateServer struct {
	client *ionoscloud.APIClient
}

func newStepCreateServer(client *ionoscloud.APIClient) *stepCreateServer {
	return &stepCreateServer{
		client: client,
	}
}

func (s *stepCreateServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	c := state.Get("config").(*Config)

	ui.Say("Creating Virtual Data Center...")
	img, err := s.getImageId(c.Image, c)
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while getting image %s", err.Error()))
		return multistep.ActionHalt
	}
	alias := ""
	if img == "" {
		alias, err = s.getImageAlias(ctx, c.Image, c.Region)
		if err != nil {
			ui.Error(fmt.Sprintf("Error occurred while getting image %s", err.Error()))
			return multistep.ActionHalt
		}
	}

	datacenter := ionoscloud.Datacenter{
		Properties: &ionoscloud.DatacenterProperties{
			Name:     ionoscloud.PtrString(c.SnapshotName),
			Location: ionoscloud.PtrString(c.Region),
		},
	}

	props := &ionoscloud.VolumeProperties{
		Type:       ionoscloud.PtrString(c.DiskType),
		Size:       ionoscloud.PtrFloat32(c.DiskSize),
		Name:       ionoscloud.PtrString(c.SnapshotName),
		ImageAlias: ionoscloud.PtrString(alias),
		Image:      ionoscloud.PtrString(img),
		SshKeys:    &[]string{string(c.Comm.SSHPublicKey)},
	}
	nic := ionoscloud.Nic{
		Properties: &ionoscloud.NicProperties{
			Name: ionoscloud.PtrString(c.SnapshotName),
			Dhcp: ionoscloud.PtrBool(true),
		},
	}
	if c.Comm.SSHPassword != "" {
		props.ImagePassword = ionoscloud.PtrString(c.Comm.SSHPassword)
	}
	server := ionoscloud.Server{
		Properties: &ionoscloud.ServerProperties{
			Name:  ionoscloud.PtrString(c.SnapshotName),
			Ram:   ionoscloud.PtrInt32(c.Ram),
			Cores: ionoscloud.PtrInt32(c.Cores),
		},
		Entities: &ionoscloud.ServerEntities{
			Volumes: &ionoscloud.AttachedVolumes{
				Items: &[]ionoscloud.Volume{
					{
						Properties: props,
					},
				},
			},
			Nics: &ionoscloud.Nics{
				Items: &[]ionoscloud.Nic{
					nic,
				},
			},
		},
	}

	// create datacenter
	datacenter, resp, err := s.client.DataCentersApi.DatacentersPost(ctx).Datacenter(datacenter).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a datacenter %s", err.Error()))
		return multistep.ActionHalt
	}

	if resp.StatusCode > 299 {
		if resp.StatusCode > 299 {
			var restError RestError
			err := json.Unmarshal([]byte(resp.Message), &restError)
			if err != nil {
				ui.Error(fmt.Sprintf("Error decoding json response: %s", err.Error()))
				return multistep.ActionHalt
			}
			if len(restError.Messages) > 0 {
				ui.Error(restError.Messages[0].Message)
			} else {
				ui.Error(resp.Message)
			}
			return multistep.ActionHalt
		}
	}

	err = s.waitTillProvisioned(ctx, resp.Header.Get("Location"), *c)
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a datacenter %s", err.Error()))
		return multistep.ActionHalt
	}

	state.Put("datacenter_id", datacenter.Id)

	// create server
	server, sResp, err := s.client.ServersApi.DatacentersServersPost(ctx, *datacenter.Id).Server(server).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a server %s", err.Error()))
		return multistep.ActionHalt
	}
	if sResp.StatusCode > 299 {
		ui.Error(fmt.Sprintf("Error occurred %s", parseErrorMessage(sResp.Message)))
		return multistep.ActionHalt
	}

	err = s.waitTillProvisioned(ctx, sResp.Header.Get("Location"), *c)
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a server %s", err.Error()))
		return multistep.ActionHalt
	}

	lanPost := ionoscloud.LanPost{
		Properties: &ionoscloud.LanPropertiesPost{
			Public: ionoscloud.PtrBool(true),
			Name:   ionoscloud.PtrString(c.SnapshotName),
		},
	}
	// create lan
	lan, lResp, err := s.client.LANsApi.DatacentersLansPost(ctx, *datacenter.Id).Lan(lanPost).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a LAN %s", err.Error()))
		return multistep.ActionHalt
	}
	if lResp.StatusCode > 299 {
		ui.Error(fmt.Sprintf("Error occurred %s", parseErrorMessage(lResp.Message)))
		return multistep.ActionHalt
	}

	err = s.waitTillProvisioned(ctx, lResp.Header.Get("Location"), *c)
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a LAN %s", err.Error()))
		return multistep.ActionHalt
	}

	// attach lan to server
	nic, nicRESP, err := s.client.LANsApi.DatacentersLansNicsPost(ctx, *datacenter.Id, *lan.Id).Nic(nic).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while attaching a NIC %s", err.Error()))
		return multistep.ActionHalt
	}

	if nicRESP.StatusCode > 299 {
		ui.Error(fmt.Sprintf("Error occurred %s", parseErrorMessage(nicRESP.Message)))
		return multistep.ActionHalt
	}

	err = s.waitTillProvisioned(ctx, nicRESP.Header.Get("Location"), *c)
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while creating a NIC %s", err.Error()))
		return multistep.ActionHalt
	}

	volumes := *server.Entities.Volumes.Items
	state.Put("volume_id", volumes[0].Id)

	server, sResp, err = s.client.ServersApi.DatacentersServersFindById(ctx, *datacenter.Id, *server.Id).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("Error occurred while finding the server %s", err.Error()))
		return multistep.ActionHalt
	}

	//server = profitbricks.GetServer(datacenter.Id, server.Id)
	// instance_id is the generic term used so that users can have access to the
	// instance id inside of the provisioners, used in step_provision.
	state.Put("instance_id", server.Id)

	nics := *server.Entities.Nics.Items
	ips := *nics[0].Properties.Ips
	state.Put("server_ip", ips[0])

	return multistep.ActionContinue
}

func (s *stepCreateServer) Cleanup(state multistep.StateBag) {
	c := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)

	ui.Say("Removing Virtual Data Center...")

	if dcId, ok := state.GetOk("datacenter_id"); ok {
		resp, err := s.client.DataCentersApi.DatacentersDelete(context.TODO(), dcId.(string)).Execute()
		if err != nil {
			ui.Error(fmt.Sprintf(
				"Error deleting Virtual Data Center. Please destroy it manually: %s", err))
		}
		if err := s.checkForErrors(resp); err != nil {
			ui.Error(fmt.Sprintf(
				"Error deleting Virtual Data Center. Please destroy it manually: %s", err))
		}
		if err := s.waitTillProvisioned(context.TODO(), resp.Header.Get("Location"), *c); err != nil {
			ui.Error(fmt.Sprintf(
				"Error deleting Virtual Data Center. Please destroy it manually: %s", err))
		}
	}
}

func (s *stepCreateServer) waitTillProvisioned(ctx context.Context, path string, config Config) error {
	waitCount := 120
	if config.Retries > 0 {
		waitCount = config.Retries
	}
	for i := 0; i < waitCount; i++ {
		status, _, err := s.client.GetRequestStatus(ctx, path)
		if err != nil {
			return err
		}
		if *status.Metadata.Status == "DONE" {
			return nil
		}
		if *status.Metadata.Status == "FAILED" {
			return errors.New(*status.Metadata.Message)
		}
		time.Sleep(1 * time.Second)
		i++
	}
	return nil
}

func (s *stepCreateServer) checkForErrors(instance *ionoscloud.APIResponse) error {
	if instance.StatusCode > 299 {
		return fmt.Errorf("error occurred %s", instance.Message)
	}
	return nil
}

type RestError struct {
	HttpStatus int       `json:"httpStatus,omitempty"`
	Messages   []Message `json:"messages,omitempty"`
}

type Message struct {
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s *stepCreateServer) getImageId(imageName string, c *Config) (string, error) {
	images, resp, err := s.client.ImagesApi.ImagesGet(context.Background()).Execute()
	if err != nil {
		return "", err
	}
	if resp.StatusCode > 299 {
		return "", errors.New("error occurred while getting images")
	}

	for i := 0; i < len(*images.Items); i++ {
		imgName := ""
		items := *images.Items
		if *items[i].Properties.Name != "" {
			imgName = *items[i].Properties.Name
		}
		diskType := c.DiskType
		if c.DiskType == "SSD" {
			diskType = "HDD"
		}
		if imgName != "" && strings.Contains(strings.ToLower(imgName), strings.ToLower(imageName)) && *items[i].Properties.ImageType == diskType && *items[i].Properties.Location == c.Region && *items[i].Properties.Public {
			return *items[i].Id, nil
		}
	}
	return "", nil
}

func (s *stepCreateServer) getImageAlias(ctx context.Context, imageAlias string, location string) (string, error) {
	if imageAlias == "" {
		return "", nil
	}

	locations, resp, err := s.client.LocationsApi.LocationsFindByRegionId(ctx, location).Execute()
	if err != nil {
		return "", err
	}
	if resp.StatusCode > 299 {
		return "", errors.New("error occurred while getting locations")
	}

	items := *locations.Items
	for i := 0; i < len(items); i++ {
		loc := items[i]
		if len(*loc.Properties.ImageAliases) > 0 {
			for _, i := range *loc.Properties.ImageAliases {
				alias := ""
				if i != "" {
					alias = i
				}
				if alias != "" && strings.EqualFold(alias, imageAlias) {
					return alias, nil
				}
			}
		}
	}

	return "", nil
}

func parseErrorMessage(raw string) (toreturn string) {
	var tmp map[string]interface{}
	if json.Unmarshal([]byte(raw), &tmp) != nil {
		return ""
	}

	for _, v := range tmp["messages"].([]interface{}) {
		for index, i := range v.(map[string]interface{}) {
			if index == "message" {
				toreturn = toreturn + i.(string) + "\n"
			}
		}
	}
	return toreturn
}
