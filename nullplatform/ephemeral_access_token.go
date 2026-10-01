package nullplatform

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResourceWithConfigure = (*accessTokenEphemeralResource)(nil)

type accessTokenEphemeralResource struct {
	client *NullClient
}

type accessTokenModel struct {
	AccessToken types.String `tfsdk:"access_token"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
}

func NewAccessTokenEphemeralResource() ephemeral.EphemeralResource {
	return &accessTokenEphemeralResource{}
}

func (r *accessTokenEphemeralResource) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_token"
}

func (r *accessTokenEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Exchanges the provider's API key for a short-lived nullplatform access token, " +
			"for tools the provider does not cover, such as the `np` CLI (which reads it from `NP_TOKEN`). " +
			"The token is never stored in plan or state, and it carries the same permissions as the API key. " +
			"Requires Terraform 1.10 or later.",
		Attributes: map[string]schema.Attribute{
			"access_token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "A freshly issued access token, sent as `Authorization: Bearer <token>`. Each run issues a new one.",
			},
			"expires_at": schema.StringAttribute{
				Computed:    true,
				Description: "When the access token expires, in RFC 3339 format (UTC). Null when the token does not declare an expiry.",
			},
		},
	}
}

func (r *accessTokenEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, _ *ephemeral.ConfigureResponse) {
	if client, ok := req.ProviderData.(*NullClient); ok {
		r.client = client
	}
}

func (r *accessTokenEphemeralResource) Open(ctx context.Context, _ ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	if r.client == nil || r.client.ApiKey == "" {
		resp.Diagnostics.AddError("Missing API Key", "Either 'api_key' or 'np_apikey' must be set. Please provide an API key for authentication.")
		return
	}

	// A client of its own, so every open issues a fresh token instead of the provider's cached one.
	issuer := &NullClient{Client: r.client.Client, ApiURL: r.client.ApiURL, ApiKey: r.client.ApiKey}
	if diags := issuer.getToken(); diags.HasError() {
		resp.Diagnostics.AddError("Failed to issue access token", diags[0].Summary)
		return
	}

	result := accessTokenModel{
		AccessToken: types.StringValue(issuer.Token.AccessToken),
		ExpiresAt:   types.StringNull(),
	}
	if exp := tokenExpiry(issuer.Token.AccessToken); !exp.IsZero() {
		result.ExpiresAt = types.StringValue(exp.UTC().Format(time.RFC3339))
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &result)...)
}

// tokenExpiry reads the exp claim without verifying: the token is only relayed, never trusted here.
func tokenExpiry(token string) time.Time {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		return time.Time{}
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return time.Time{}
	}
	return exp.Time
}
