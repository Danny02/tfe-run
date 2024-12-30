package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danny02/tfe-run/gha"
	tfe "github.com/hashicorp/go-tfe"
)

type input struct {
	Token             string `gha:"token,required"`
	Organization      string `gha:"organization,required"`
	Workspace         string `gha:"workspace,required"`
	Message           string
	Directory         string
	Type              string
	Targets           string
	Replacements      string
	WaitForCompletion bool   `gha:"wait-for-completion"`
	TfVars            string `gha:"tf-vars"`
}

type ClientConfig struct {
	// Token used to communicate with the Terraform Cloud API. Must be a user
	// or team API token.
	Token string
	// The organization on Terraform Cloud.
	Organization string
	// The workspace on Terraform Cloud.
	Workspace string
}

// Client is used to interact with the Run API of a single workspace on
// Terraform Cloud.
type Client struct {
	client    *tfe.Client
	workspace *tfe.Workspace
}

// NewClient creates a Client from ClientConfig.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	config := &tfe.Config{
		Token: cfg.Token,
	}
	tfeClient, err := tfe.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("could not create a new TFE tfeClient: %w", err)
	}

	w, err := tfeClient.Workspaces.Read(ctx, cfg.Organization, cfg.Workspace)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve workspace '%v/%v': %w", cfg.Organization, cfg.Workspace, err)
	}

	c := Client{
		client:    tfeClient,
		workspace: w,
	}
	return &c, nil
}

// RunOptions groups all options available when creating a new run.
type RunOptions struct {
	// Message to use as name of the run. This field is optional.
	Message *string
	// The directory that is uploaded to Terraform Cloud, respects
	// .terraformignore. Defaults to the current directory.
	Directory *string
	// The type of run to schedule.
	Type RunType
	// A list of resource addresses that are passed to the -target flag. For
	// more details, check https://www.terraform.io/docs/commands/plan.html#resource-targeting
	TargetAddrs []string
	// A list of resource addresses that are passed to the -replace flag. For
	// more details, check https://developer.hashicorp.com/terraform/cli/commands/plan#replace-address
	ReplaceAddrs []string
	// Whether we should wait for the non-speculative run to be applied. This
	// will block until the run is finished.
	WaitForCompletion bool
	// Contents of a auto.tfvars file that will be uploaded to Terraform Cloud.
	// This can be used to set temporary Terraform variables. These variables
	// will not be preserved across runs.
	TfVars *string
}

// RunType describes the type of run.
type RunType int

// Declaration of run types.
const (
	RunTypePlan RunType = iota
	RunTypeApply
	RunTypeDestroy
)

// RunOutput holds the data that is generated by a run.
type RunOutput struct {
	// URL to the run on Terraform Cloud.
	RunURL string
	// Whether this run has changes. After a speculative plan this would
	// indicate whether an apply would cause changes, after a non-speculative
	// plan this indicates whether the run has caused any changes.
	// This is not populated for non-speculative runs on workspaces that do not
	// have auto-apply configured or when WaitForCompletion is not set.
	HasChanges *bool
}

// Run creates a new run on Terraform Cloud.
//
// If RunOptions.WaitForCompletion is set this method will block until the run
// is finished, except if the run is non-speculative and the workspace has
// disabled auto-apply (to avoid blocking indefinitely).
// If the run does not complete within one hour, ErrTimeout is returned. This
// will not cancel the remote operation.
func (c *Client) Run(ctx context.Context, options RunOptions) (output RunOutput, err error) {
	cvOptions := tfe.ConfigurationVersionCreateOptions{
		// Don't automatically queue the new run, we want to create the run
		// manually to be able to set the message.
		AutoQueueRuns: tfe.Bool(false),
		Speculative:   tfe.Bool(options.Type == RunTypePlan),
	}
	cv, err := c.client.ConfigurationVersions.Create(ctx, c.workspace.ID, cvOptions)
	if err != nil {
		if err == tfe.ErrResourceNotFound {
			err = fmt.Errorf("could not create configuration version (404 not found), this might happen if you are not using a user or team API token")
		} else {
			err = fmt.Errorf("could not create a new configuration version: %w", err)
		}
		return
	}

	var dir string
	if options.Directory != nil {
		dir = *options.Directory
	} else {
		dir = "./"
	}

	if options.TfVars != nil {
		// Creating a *.auto.tfvars file that is uploaded with the rest of the
		// code is the easiest way to temporarily set a variable. The Terraform
		// Cloud API only allows setting workspace variables. These variables
		// are persistent across runs which might cause undesired side-effects.
		varsFile := filepath.Join(dir, c.workspace.WorkingDirectory, "run.auto.tfvars")

		fmt.Printf("Creating temporary variables file %v\n", varsFile)

		err = os.WriteFile(varsFile, []byte(*options.TfVars), 0644)
		if err != nil {
			err = fmt.Errorf("could not create run.auto.tfvars: %w", err)
			return
		}

		defer func() {
			err := os.Remove(varsFile)
			if err != nil {
				fmt.Printf("Could not remove run.auto.tfvars: %v", err)
			}
		}()
	}

	fmt.Print("Uploading directory...\n")

	err = c.client.ConfigurationVersions.Upload(ctx, cv.UploadURL, dir)
	if err != nil {
		err = fmt.Errorf("could not upload directory '%v': %w", options.Directory, err)
		return
	}

	fmt.Print("Done uploading.\n")

	// wait until configuration version has status Uploaded
	// this is also done in the Terraform implementation: https://github.com/hashicorp/terraform/blob/v0.13.1/backend/remote/backend_plan.go#L204-L231
	err = pollWithContext(ctx, 5*time.Second, func() (bool, error) {
		cv, err = c.client.ConfigurationVersions.Read(ctx, cv.ID)
		if err != nil {
			return false, fmt.Errorf("could not get current configuration version: %w", err)
		}
		if cv.Status == tfe.ConfigurationErrored {
			return false, fmt.Errorf("configuration version errored: %v - %v", cv.Error, cv.ErrorMessage)
		}
		return cv.Status == tfe.ConfigurationUploaded, nil
	})
	if err != nil {
		err = fmt.Errorf("uploading configuration version failed: %w", err)
		return
	}

	fmt.Print("Configuration version is uploaded and processed.\n")

	var r *tfe.Run

	rOptions := tfe.RunCreateOptions{
		Workspace:            c.workspace,
		ConfigurationVersion: cv,
		IsDestroy:            tfe.Bool(options.Type == RunTypeDestroy),
		TargetAddrs:          options.TargetAddrs,
		ReplaceAddrs:         options.ReplaceAddrs,
		Message:              options.Message,
	}
	r, err = c.client.Runs.Create(ctx, rOptions)
	if err != nil {
		err = fmt.Errorf("could not create run: %w", err)
		return
	}

	output.RunURL = fmt.Sprintf(
		"https://app.terraform.io/app/%v/workspaces/%v/runs/%v",
		c.workspace.Organization.Name, c.workspace.Name, r.ID,
	)

	fmt.Printf("Run %v has been queued\n", r.ID)
	fmt.Printf("View the run online:\n")
	fmt.Printf("%v\n", output.RunURL)

	if !options.WaitForCompletion {
		return
	}

	// If auto apply isn't enabled a run could hang for a long time, even if
	// the run itself wouldn't change anything the previous run could still be
	// blocked while waiting for confirmation.
	// Speculative runs/plans can always continue.
	if !(options.Type == RunTypePlan) && !c.workspace.AutoApply {
		fmt.Print("Auto apply isn't enabled, won't wait for completion.\n")
		return
	}

	var prevStatus tfe.RunStatus

	err = pollWithContext(ctx, 60*time.Minute, func() (bool, error) {
		r, err = c.client.Runs.Read(ctx, r.ID)
		if err != nil {
			return false, fmt.Errorf("could not read run: %w", err)
		}

		if prevStatus != r.Status {
			fmt.Printf("Run status: %v\n", prettyPrint(r.Status))
			prevStatus = r.Status
		}

		return isEndStatus(r.Status), nil
	})
	if err != nil {
		err = fmt.Errorf("waiting for completion of run failed: %w", err)
		return
	}

	output.HasChanges = tfe.Bool(r.HasChanges)

	switch r.Status {
	case tfe.RunPlannedAndFinished:
		fmt.Println("Run is planned and finished.")
	case tfe.RunApplied:
		fmt.Println("Run has been applied!")
	default:
		err = fmt.Errorf("run %v finished with status %v", r.ID, prettyPrint(r.Status))
	}

	return
}

func isEndStatus(r tfe.RunStatus) bool {
	// Run statuses: https://pkg.go.dev/github.com/hashicorp/go-tfe?tab=doc#RunStatus
	// Documentation: https://www.terraform.io/docs/cloud/api/run.html#run-states
	switch r {
	case
		tfe.RunPolicySoftFailed,
		tfe.RunPlannedAndFinished,
		tfe.RunApplied,
		tfe.RunDiscarded,
		tfe.RunErrored,
		tfe.RunCanceled:
		return true
	}
	return false
}

func prettyPrint(r tfe.RunStatus) string {
	return strings.ReplaceAll(string(r), "_", " ")
}

type minimalTerraformState struct {
	Outputs map[string]terraformOutput `json:"outputs"`
}

type terraformOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// GetTerraformOutputs retrieves the outputs from the current Terraform state.
func (c *Client) GetTerraformOutputs(ctx context.Context) (map[string]string, error) {
	s, err := c.client.StateVersions.ReadCurrent(ctx, c.workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("could not get current state: %w", err)
	}

	bytes, err := c.client.StateVersions.Download(ctx, s.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("could not download state: %w", err)
	}

	var state minimalTerraformState
	err = json.Unmarshal(bytes, &state)
	if err != nil {
		return nil, fmt.Errorf("could not parse state: %w", err)
	}

	outputs := make(map[string]string)
	for k, v := range state.Outputs {
		outputs[k] = v.Value
	}

	fmt.Printf("Outputs from current state:\n")
	for k, v := range outputs {
		fmt.Printf(" - %v: %v\n", k, v)
	}

	return outputs, nil
}

var (
	// ErrTimeout is returned when an operation timed out.
	ErrTimeout = errors.New("timed out while polling")
)

// pollWithContext will execute pollFn every 500 milliseconds until either
// pollFn returns (true, nil) or (false, err). If more than timeout time has
// elapsed since the start of pollWithContext, ErrTimeout is returned.
func pollWithContext(ctx context.Context, timeout time.Duration, pollFn func() (success bool, err error)) error {
	start := time.Now()

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-time.After(500 * time.Millisecond):
			success, err := pollFn()
			if err != nil || success {
				return err
			}

			if time.Since(start) > timeout {
				return ErrTimeout
			}
		}
	}
}

func main() {
	var input input
	var err error

	if !gha.InGitHubActions() {
		exitWithError(errors.New("tfe-run should only be run within GitHub Actions"))
	}

	err = gha.PopulateFromInputs(&input)
	if err != nil {
		exitWithError(fmt.Errorf("could not read inputs: %w", err))
	}

	runType := asRunType(input.Type)

	ctx := context.Background()

	cfg := ClientConfig{
		Token:        input.Token,
		Organization: input.Organization,
		Workspace:    input.Workspace,
	}
	c, err := NewClient(ctx, cfg)
	if err != nil {
		exitWithError(err)
	}

	options := RunOptions{
		Message:           notEmptyOrNil(input.Message),
		Directory:         notEmptyOrNil(input.Directory),
		Type:              runType,
		TargetAddrs:       notAllEmptyOrNil(strings.Split(input.Targets, "\n")),
		ReplaceAddrs:      notAllEmptyOrNil(strings.Split(input.Replacements, "\n")),
		WaitForCompletion: input.WaitForCompletion,
		TfVars:            notEmptyOrNil(input.TfVars),
	}
	output, err := c.Run(ctx, options)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	gha.WriteOutput("run-url", output.RunURL)
	if output.HasChanges != nil {
		gha.WriteOutput("has-changes", strconv.FormatBool(*output.HasChanges))
	}

	outputs, err := c.GetTerraformOutputs(ctx)
	if err != nil {
		exitWithError(err)
	}

	for k, v := range outputs {
		gha.WriteOutput(fmt.Sprintf("tf-%v", k), v)
	}
}

func asRunType(s string) RunType {
	switch s {
	case "apply":
		return RunTypeApply
	case "plan":
		return RunTypePlan
	case "destroy":
		return RunTypeDestroy
	}
	exitWithError(fmt.Errorf("Type \"%s\" is not supported, must be plan, apply or destroy", s))
	return 0
}

func notEmptyOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func notAllEmptyOrNil(slice []string) []string {
	for _, s := range slice {
		if s != "" {
			return slice
		}
	}
	return nil
}

func exitWithError(err error) {
	fmt.Printf("Error: %v", err)
	os.Exit(1)
}
