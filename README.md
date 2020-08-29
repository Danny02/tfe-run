# `tfe-run` Action

![CI](https://github.com/kvrhdn/tfe-run/workflows/CI/badge.svg)
![Integration](https://github.com/kvrhdn/tfe-run/workflows/Integration/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/kvrhdn/tfe-run)](https://goreportcard.com/report/github.com/kvrhdn/tfe-run)

This GitHub Action creates a new run on Terraform Cloud. Integrate Terraform Cloud into your GitHub Actions workflow.

This action creates runs using [the Terraform Cloud API][tfe-api] which provides more flexibility than using the CLI. Namely, you can:
- define your own message (no more _"Queued manually using Terraform"_)
- provide as many variables as you want
- access the outputs from the Terraform state

Internally, we leverage [the official Go API client from Hashicorp][go-tfe].

[tfe-api]: https://www.terraform.io/docs/cloud/run/api.html
[go-tfe]: https://github.com/hashicorp/go-tfe/

## How to use it

```yaml
- uses: kvrhdn/tfe-run@v1
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
- uses: kvrhdn/tfe-run@v1
  with:
    # Token used to communicate with the Terraform Cloud API. Must be a user or
    # team api token.
    token: ${{ secrets.TFE_TOKEN }}

    # Name of the organization on Terraform Cloud. Defaults to the GitHub
    # organization name.
    organization: kvrhdn

    # Name of the workspace on Terraform Cloud.
    workspace: tfe-run

    # Optional message to use as name of the run.
    message: |
      Run triggered using tfe-run (commit: ${{ github.SHA }})

    # The directory that is uploaded to Terraform Enterprise, defaults to the
    # repository root. Respsects .terraformignore.
    directory: integration/

    # Whether to run a speculative run. A speculative run can not be applied
    # and is useful for creating previews of changes.
    speculative: false

    # Whether we should wait for the plan or run to be applied. This will block
    # until the run is finished.
    wait-for-completion: true

    # The contents of a auto.tfvars file that will be uploaded to Terraform
    # Cloud. This can be used to set Terraform variables.
    tf-vars: |
      run_number = ${{ github.run_number }}

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
`directory`    |          | The directory that is uploaded to Terraform Enterprise, defaults to repository root. Respects .terraformignore. | string | `./`
`speculative`  |          | Whether to run [a speculative run][tfe-speculative-run]. A speculative run can not be applied and is useful for creating previews of changes. | bool   | `false`
`wait-for-completion` |   | Whether we should wait for the plan or run to be applied. This will block until the run is finished.            | string | `false`
`tf-vars`      |          | The contents of a auto.tfvars file that will be uploaded to Terraform Cloud.                                    | string |

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
