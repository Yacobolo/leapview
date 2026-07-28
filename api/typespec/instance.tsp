using Http;
using TypeSpec.OpenAPI;
using LeapViewAPI.Protocol;

@route("/api/v1")
namespace LeapViewAPI;

model InstanceResponse {
  environment: string;
}

@tag("Instance")
@summary("Get this LeapView instance identity")
@route("/instance")
@get
@apigen.authz(#{ mode: "authenticated" })
@operationId("getInstance")
op getInstance(): OkJson<InstanceResponse> | CommonErrors;
