# `tfe-run` Action

[![CI](https://github.com/danny02/tfe-run/workflows/CI/badge.svg)](https://github.com/danny02/tfe-run/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/danny02/tfe-run)](https://goreportcard.com/report/github.com/danny02/tfe-run)

This GitHub Action creates a new run on Terraform Cloud. Integrate Terraform Cloud into your GitHub Actions workflow.

This action creates runs using [the Terraform Cloud API][tfe-api] which provides more flexibility than using the CLI. Namely, you can:
- define your own message (no more _"Queued manually using Terraform"_)
- access the outputs from the Terraform state

Internally, we leverage [the official Go API client from Hashicorp][go-tfe].

[tfe-api]: https://www.terraform.io/docs/cloud/run/api.html
[go-tfe]: https://github.com/hashicorp/go-tfe/

## How to use it

```yaml
- uses: danny02/tfe-run@v1
  with:
    token: ${{ secrets.TFE_TOKEN }}
    workspace: tfe-run
    message: |
      Run triggered using tfe-run (commit: ${{ github.SHA }})
  id: tfe-run

... next steps can access the run URL with ${{ steps.tfe-run.outputs.run-url }}
```

Full option list:

```yaml
- uses: danny02/tfe-run@v1
  with:
    # Token used to communicate with the Terraform Cloud API. Must be a user or
    # team api token.
    token: ${{ secrets.TFE_TOKEN }}

    # Name of the organization on Terraform Cloud. Defaults to the GitHub
    # organization name.
    organization: danny02

    # Name of the workspace on Terraform Cloud.
    workspace: tfe-run

    # Optional message to use as name of the run.
    message: |
      Run triggered using tfe-run (commit: ${{ github.SHA }})

    # The type of run, allowed options are 'plan', 'apply' and 'destroy'.
    type: apply

    # An optional list of resource addresses to target. Should be a list of
    # strings separated by new lines.
    #
    # For more information about resource targeting, check https://developer.hashicorp.com/terraform/cli/commands/plan#resource-targeting
    targets: |
        resource.name

    # An optional list of resource addresses to replace. Should be a list of
    # strings separated by new lines.
    #
    # For more information about resource targeting, check https://developer.hashicorp.com/terraform/cli/commands/plan#replace-address
    replacements: |
        resource.name

    # Whether we should wait for the plan or run to be applied. This will block
    # until the run is finished.
    wait-for-completion: true

  # Optionally, assign this step an ID so you can refer to the outputs from the
  # action with ${{ steps.<id>.outputs.<output variable> }}
  id: tfe-run
```

### Inputs

Name           | Required | Description                                                                                                     | Type   | Default
---------------|----------|-----------------------------------------------------------------------------------------------------------------|--------|--------
`token`        | yes      | Token used to communicating with the Terraform Cloud API. Must be [a user or team api token][tfe-tokens].       | string | 
`organization` |          | Name of the organization on Terraform Cloud.                                                                    | string | The repository owner
`workspace`    | yes      | Name of the workspace on Terraform Cloud.                                                                       | string |
`message`      |          | Optional message to use as name of the run.                                                                     | string | _Queued by GitHub Actions (commit: $GITHUB_SHA)_
`type`         |          | The type of run, allowed options are 'plan', 'apply' and 'destroy'.                                             | string | `apply`
`targets`      |          | An optional list of resource addresses to target. Should be a list of strings separated by new lines.           | string |
`wait-for-completion` |   | Whether we should wait for the plan or run to be applied. This will block until the run is finished.            | string | `true`
`print-outputs`| | Whether terraform outputs should be printed  | string | `true`

[tfe-tokens]: https://www.terraform.io/docs/cloud/users-teams-organizations/api-tokens.html
[tfe-speculative-run]: https://www.terraform.io/docs/cloud/run/index.html#speculative-plans

### Outputs

Name          | Description                                                                                       | Type
--------------|---------------------------------------------------------------------------------------------------|-----
`run-url`     | URL of the run on Terraform Cloud                                                                 | string
`has-changes` | Whether the run has changes.                                                                      | bool (`'true'` or `'false'`)
`tf-**`       | Outputs from the current Terraform state, prefixed with `tf-`. Only set for non-speculative runs. | string

## License

This Action is distributed under the terms of the MIT license, see [LICENSE](./LICENSE) for details.

## Development

For running tfe-run locally, see [development.md](./doc/development.md).

For creating new release, see [release-procedure.md](./doc/release-procedure.md).
