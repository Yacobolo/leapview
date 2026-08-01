#!/bin/zsh

source "${0:A:h}/lib.zsh"
load_secrets
ensure_ssh_key

public_ip="$(curl -fsS --max-time 10 https://api.ipify.org)"
[[ "${public_ip}" == <->.<->.<->.<-> ]] || { print -u2 'could not determine operator IPv4'; exit 1; }

workspace_url="https://app.terraform.io/api/v2/organizations/Flid/workspaces/leapview-rtest-poc"
response="${RTEST_TMP_DIR}/hcp-workspace.json"
http_status="$(curl -sS -o "${response}" -w '%{http_code}' \
  -H "Authorization: Bearer ${HCP_API_TOKEN}" \
  -H 'Content-Type: application/vnd.api+json' \
  "${workspace_url}")"
if [[ "${http_status}" == 404 ]]; then
  http_status="$(curl -sS -o "${response}" -w '%{http_code}' -X POST \
    -H "Authorization: Bearer ${HCP_API_TOKEN}" \
    -H 'Content-Type: application/vnd.api+json' \
    --data '{"data":{"type":"workspaces","attributes":{"name":"leapview-rtest-poc","execution-mode":"local","auto-apply":false}}}' \
    'https://app.terraform.io/api/v2/organizations/Flid/workspaces')"
fi
[[ "${http_status}" == 2* ]] || { print -u2 "HCP workspace request failed with HTTP ${http_status}"; exit 1; }
workspace_id="$(jq -er '.data.id' "${response}")"
curl -fsS -o /dev/null -X PATCH \
  -H "Authorization: Bearer ${HCP_API_TOKEN}" \
  -H 'Content-Type: application/vnd.api+json' \
  --data "{\"data\":{\"type\":\"workspaces\",\"id\":\"${workspace_id}\",\"attributes\":{\"execution-mode\":\"local\"}}}" \
  "https://app.terraform.io/api/v2/workspaces/${workspace_id}"

jq -n --arg key "${RTEST_SSH_KEY}.pub" --arg cidr "${public_ip}/32" \
  '{ssh_public_key_path:$key,ssh_allowed_cidrs:[$cidr]}' > "${RTEST_TMP_DIR}/terraform.auto.tfvars.json"

terraform -chdir="${RTEST_INFRA_DIR}" init \
  -backend-config='organization=Flid' \
  -backend-config='workspaces.name=leapview-rtest-poc'
terraform -chdir="${RTEST_INFRA_DIR}" apply -auto-approve \
  -var-file="${RTEST_TMP_DIR}/terraform.auto.tfvars.json"

print "provisioned $(server_ip); SSH is restricted to ${public_ip}/32"
