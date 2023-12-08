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

type stepTakeSnapshot struct {
	client *ionoscloud.APIClient
}

func newStepTakeSnapshot(client *ionoscloud.APIClient) *stepTakeSnapshot {
	return &stepTakeSnapshot{
		client: client,
	}
}

func (s *stepTakeSnapshot) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	c := state.Get("config").(*Config)

	ui.Say("Creating ProfitBricks snapshot...")

	dcId := state.Get("datacenter_id").(string)
	volumeId := state.Get("volume_id").(string)
	serverId := state.Get("instance_id").(string)

	comm, _ := state.Get("communicator").(packersdk.Communicator)
	if comm == nil {
		ui.Error("no communicator found")
		return multistep.ActionHalt
	}

	/* sync fs changes from the provisioning step */
	os, err := s.getOs(ctx, dcId, serverId)
	if err != nil {
		ui.Error(fmt.Sprintf("an error occurred while getting the server os: %s", err.Error()))
		return multistep.ActionHalt
	}
	ui.Say(fmt.Sprintf("Server OS is %s", os))

	switch strings.ToLower(os) {
	case "linux":
		ui.Say("syncing file system changes")
		if err := s.syncFs(ctx, comm); err != nil {
			ui.Error(fmt.Sprintf("error syncing fs changes: %s", err.Error()))
			return multistep.ActionHalt
		}
	}

	snapshot, resp, err := s.client.VolumesApi.DatacentersVolumesCreateSnapshotPost(ctx, dcId, volumeId).Execute()
	if err != nil {
		ui.Error(fmt.Sprintf("An error occurred while creating a snapshot: %s", err.Error()))
		return multistep.ActionHalt
	}
	if resp.StatusCode > 299 {
		var restError RestError
		if err := json.Unmarshal([]byte(resp.Message), &restError); err != nil {
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if len(restError.Messages) > 0 {
			ui.Error(restError.Messages[0].Message)
		} else {
			ui.Error(resp.Message)
		}

		return multistep.ActionHalt
	}

	//snapshot := profitbricks.CreateSnapshot(dcId, volumeId, c.SnapshotName, "")\

	state.Put("snapshotname", c.SnapshotName)

	ui.Say(fmt.Sprintf("Creating a snapshot for %s/volumes/%s", dcId, volumeId))

	err = s.waitForRequest(ctx, resp.Header.Get("Location"), *c, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("An error occurred while waiting for the request to be done: %s", err.Error()))
		return multistep.ActionHalt
	}

	err = s.waitTillSnapshotAvailable(ctx, *snapshot.Id, *c, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("An error occurred while waiting for the snapshot to be created: %s", err.Error()))
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *stepTakeSnapshot) Cleanup(_ multistep.StateBag) {
}

func (s *stepTakeSnapshot) waitForRequest(ctx context.Context, path string, config Config, ui packersdk.Ui) error {
	ui.Say(fmt.Sprintf("Watching request %s", path))
	waitCount := 50
	var waitInterval = 10 * time.Second
	if config.Retries > 0 {
		waitCount = config.Retries
	}
	done := false
	for i := 0; i < waitCount; i++ {
		request, resp, err := s.client.GetRequestStatus(ctx, path)
		if err != nil {
			return err
		}
		ui.Say(fmt.Sprintf("request status = %s", *request.Metadata.Status))
		if *request.Metadata.Status == "DONE" {
			done = true
			break
		}
		if *request.Metadata.Status == "FAILED" {
			return fmt.Errorf("request failed: %s", resp.Message)
		}
		time.Sleep(waitInterval)
		i++
	}

	if !done {
		return fmt.Errorf("request not fulfilled after waiting %d seconds",
			int64(waitCount)*int64(waitInterval)/int64(time.Second))
	}
	return nil
}

func (s *stepTakeSnapshot) waitTillSnapshotAvailable(ctx context.Context, id string, config Config, ui packersdk.Ui) error {
	waitCount := 50
	var waitInterval = 10 * time.Second
	if config.Retries > 0 {
		waitCount = config.Retries
	}
	done := false
	ui.Say(fmt.Sprintf("waiting for snapshot %s to become available", id))

	for i := 0; i < waitCount; i++ {
		snapshot, resp, err := s.client.SnapshotsApi.SnapshotsFindById(ctx, id).Execute()
		if err != nil {
			return err
		}
		ui.Say(fmt.Sprintf("snapshot status = %s", *snapshot.Metadata.State))
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s", resp.Message)
		}
		if *snapshot.Metadata.State == "AVAILABLE" {
			done = true
			break
		}
		time.Sleep(waitInterval)
		i++
		ui.Say(fmt.Sprintf("... still waiting, %d seconds have passed", int64(waitInterval)*int64(i)))
	}

	if !done {
		return fmt.Errorf("snapshot not created after waiting %d seconds",
			int64(waitCount)*int64(waitInterval)/int64(time.Second))
	}

	ui.Say("snapshot created")
	return nil
}

func (s *stepTakeSnapshot) syncFs(ctx context.Context, comm packersdk.Communicator) error {
	cmd := &packersdk.RemoteCmd{
		Command: "sync",
	}
	if err := comm.Start(ctx, cmd); err != nil {
		return err
	}
	if cmd.Wait() != 0 {
		return fmt.Errorf("sync command exited with code %d", cmd.ExitStatus())
	}
	return nil
}

func (s *stepTakeSnapshot) getOs(ctx context.Context, dcId string, serverId string) (string, error) {

	server, resp, err := s.client.ServersApi.DatacentersServersFindById(ctx, dcId, serverId).Execute()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New(resp.Message)
	}

	if server.Properties.BootVolume == nil {
		return "", errors.New("no boot volume found on server")
	}

	volumeId := *server.Properties.BootVolume.Id
	volume, resp, err := s.client.VolumesApi.DatacentersVolumesFindById(ctx, dcId, volumeId).Execute()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New(resp.Message)
	}

	return *volume.Properties.LicenceType, nil
}
