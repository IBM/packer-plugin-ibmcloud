# POC: IAM Token Exchange Authentication

## Problem

The user has an **existing access token** (from CI/CD, federated login, etc.) and cannot
use an API key. To call IBM Cloud VPC APIs they need a **scoped token** tied to a specific
service identity (a CRN). IBM IAM provides a token-exchange grant for exactly this.

The token expires in ~1 hour. A Packer build can run longer, so the token must be
re-fetched at a safe, predictable point — without background goroutines or expiry math.

---

## The Token Exchange Request

This is the exact HTTP call that fetches a fresh scoped token:

```
POST https://iam.cloud.ibm.com/identity/token
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ibm:params:oauth:grant-type:iam-authz
  &access_token=<caller's existing token>
  &desired_iam_id=crn:v1:bluemix:public:cloud-object-storage:global:a/<account>:<instance>::
```

The response is JSON. The field we need is `access_token`.

---

## New Config Fields

Three new optional fields added to `Config`:

| Template key             | Go field              | Required?                          | Default                                        |
|--------------------------|-----------------------|------------------------------------|------------------------------------------------|
| `iam_access_token`       | `IAMAccessToken`      | If not using `api_key`             | —                                              |
| `iam_desired_iam_id`     | `IAMDesiredIAMID`     | If `iam_access_token` is set       | —                                              |
| `iam_token_exchange_url` | `IAMTokenExchangeURL` | No                                 | `https://iam.cloud.ibm.com/identity/token`     |

**Validation rules in `Prepare()`:**

- `api_key` and `iam_access_token` are mutually exclusive — exactly one must be present
- If `iam_access_token` is set, `iam_desired_iam_id` must also be set
- `iam_desired_iam_id` without `iam_access_token` is an error

---

## New Functions in `step_create_vpc_service_instance.go`

### `fetchAccessToken(accessToken, desiredIAMID, exchangeURL string) (string, error)`

Does exactly the POST above:

1. Build an `application/x-www-form-urlencoded` body with the three fields
2. POST to `exchangeURL`
3. Read the response body
4. Parse JSON, return the `access_token` field
5. Any non-200 response or parse failure returns an error → build halts

### `newAuthenticator(config Config) (core.Authenticator, error)`

Single decision point, called every time the VPC service is initialised:

```
if iam_access_token is set
    → call fetchAccessToken → get fresh scoped token
    → return BearerTokenAuthenticator{ scoped_token }
else
    → return IamAuthenticator{ api_key }   ← unchanged existing path
```

---

## Where the Token Gets Refreshed — Naturally

`StepCreateVPCServiceInstance` already runs **twice** in every pipeline:

```
[1] StepCreateVPCServiceInstance   ← fetchAccessToken called here (#1)
    stepVerifyInput
    stepGetSubnetInfo
    stepGetBaseImageID
    stepCreateSshKeyPair
    stepCreateSshKeyVPC
    stepCreateInstance
    stepWaitforInstance
    stepGetIP
    stepCreateSecurityGroupRules
    StepConnect + StepProvision     ← provisioner runs here (can take 30+ min)
[2] StepCreateVPCServiceInstance   ← fetchAccessToken called here (#2), fresh token
    stepRebootInstance
    stepCaptureImage
```

Because `newAuthenticator` is called inside `Run`, a fresh token is fetched
automatically at each invocation. No timers, no background goroutines, no expiry tracking.

---

## Files Touched

| File                                  | What changes                                                      |
|---------------------------------------|-------------------------------------------------------------------|
| `config.go`                           | Add 3 fields + validation rules in `Prepare()`                   |
| `step_create_vpc_service_instance.go` | Add `fetchAccessToken` + `newAuthenticator`, wire into `Run`     |

**Nothing else changes.**
`step_verify_input.go`, `step_capture_image.go`, and `verify_encryption_key.go` all use
the `vpcService` already stored in state — which already carries the right authenticator.
They are untouched.

---

## Example Template Usage

```hcl
source "ibmcloud-vpc" "example" {
  # no api_key needed
  iam_access_token     = "${env("MY_ACCESS_TOKEN")}"
  iam_desired_iam_id   = "crn:v1:bluemix:public:cloud-object-storage:global:a/59bcbfa6ea2f006b4ed7094c1a08dcdd:1a0ec336-f391-4091-a6fb-5e084a4c56f4::"
  # iam_token_exchange_url is optional — defaults to https://iam.cloud.ibm.com/identity/token

  region    = "us-south"
  subnet_id = "..."
  ...
}
```

---

## What This POC Validates

The `poc/` folder contains a standalone Go program (`main.go`) that:

1. Reads `MY_ACCESS_TOKEN`, `DESIRED_IAM_ID`, and optionally `TOKEN_EXCHANGE_URL`
   from environment variables
2. Calls `fetchAccessToken` with those values
3. Prints the scoped token on success, or the error on failure

Run it with:

```sh
export MY_ACCESS_TOKEN="..."
export DESIRED_IAM_ID="crn:v1:bluemix:public:..."

cd poc/
go run .
```

A successful response confirms the token exchange works before wiring it into the
full builder.
