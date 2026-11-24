package backend

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(b *fastly.Backend) NestedModel {
	m := NestedModel{}

	if b == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(b.Name))
	m.Address = types.StringValue(fastly.ToValue(b.Address))
	m.Port = service.Int64PointerOrDefault(b.Port, DefaultPort)
	m.AutoLoadbalance = service.BoolPointerOrDefault(b.AutoLoadbalance, DefaultAutoLoadbalance)
	m.BetweenBytesTimeout = service.Int64PointerOrDefault(b.BetweenBytesTimeout, DefaultBetweenBytesTimeout)
	m.ConnectTimeout = service.Int64PointerOrDefault(b.ConnectTimeout, DefaultConnectTimeout)
	m.FirstByteTimeout = service.Int64PointerOrDefault(b.FirstByteTimeout, DefaultFirstByteTimeout)
	m.HealthCheck = service.StringPointerOrDefault(b.HealthCheck, DefaultHealthCheck)
	m.KeepaliveTime = service.Int64PointerOrNull(b.KeepAliveTime)
	m.MaxConn = service.Int64PointerOrDefault(b.MaxConn, DefaultMaxConn)
	m.MaxLifetime = service.Int64PointerOrDefault(b.MaxLifetime, DefaultMaxLifetime)
	m.MaxTLSVersion = service.StringPointerOrDefault(b.MaxTLSVersion, DefaultMaxTLSVersion)
	m.MaxUse = service.Int64PointerOrDefault(b.MaxUse, DefaultMaxUse)
	m.MinTLSVersion = service.StringPointerOrDefault(b.MinTLSVersion, DefaultMinTLSVersion)
	m.OverrideHost = service.StringPointerOrDefault(b.OverrideHost, DefaultOverrideHost)
	m.PreferIPv6 = service.BoolPointerOrDefault(b.PreferIPv6, DefaultPreferIPv6)
	m.RequestCondition = service.StringPointerOrDefault(b.RequestCondition, DefaultRequestCondition)
	m.ShareKey = service.StringPointerOrDefault(b.ShareKey, DefaultShareKey)
	m.Shield = service.StringPointerOrDefault(b.Shield, DefaultShield)
	m.SSLCACert = service.StringPointerOrDefault(b.SSLCACert, DefaultSSLCACert)
	m.SSLCertHostname = service.StringPointerOrDefault(b.SSLCertHostname, DefaultSSLCertHostname)
	m.SSLCheckCert = service.BoolPointerOrDefault(b.SSLCheckCert, DefaultSSLCheckCert)
	m.SSLCiphers = service.StringPointerOrDefault(b.SSLCiphers, DefaultSSLCiphers)
	// Absent client credentials should be represented as a null optional
	// object, not an object containing two empty strings. The latter is returned by
	// the API for ordinary non-mTLS backends but Terraform plans null when the
	// nested object is omitted from HCL, causing a spurious service update.
	// Keep an object whenever either credential has a non-empty value: that
	// must remain visible so an unsafe or redacted migration still fails.
	cert := service.StringPointerOrDefault(b.SSLClientCert, DefaultSSLClientCert)
	key := service.StringPointerOrDefault(b.SSLClientKey, DefaultSSLClientKey)
	if cert.ValueString() == "" && key.ValueString() == "" {
		m.SSLClientSecrets = types.ObjectNull(sslClientSecretsAttributeTypes)
	} else {
		m.SSLClientSecrets = NewSSLClientSecretsObject(cert, key)
	}
	m.SSLSNIHostname = service.StringPointerOrDefault(b.SSLSNIHostname, DefaultSSLSNIHostname)
	m.UseSSL = service.BoolPointerOrDefault(b.UseSSL, DefaultUseSSL)
	m.Weight = service.Int64PointerOrDefault(b.Weight, DefaultWeight)

	if b.Comment != nil && *b.Comment != "" {
		m.Comment = types.StringValue(*b.Comment)
	} else {
		m.Comment = types.StringNull()
	}

	return m
}

func flatten(ctx context.Context, b *fastly.Backend, m *Model) {
	if b == nil {
		tflog.Warn(ctx, "flatten called with nil backend")
		return
	}

	id := fastly.ToValue(b.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(b.ServiceVersion)) + "-" + fastly.ToValue(b.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(b.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(b.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(b)

	tflog.Debug(ctx, "Flattened service backend state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
