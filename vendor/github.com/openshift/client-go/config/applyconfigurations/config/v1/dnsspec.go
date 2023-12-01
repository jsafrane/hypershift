// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

// DNSSpecApplyConfiguration represents an declarative configuration of the DNSSpec type for use
// with apply.
type DNSSpecApplyConfiguration struct {
	BaseDomain  *string                            `json:"baseDomain,omitempty"`
	PublicZone  *DNSZoneApplyConfiguration         `json:"publicZone,omitempty"`
	PrivateZone *DNSZoneApplyConfiguration         `json:"privateZone,omitempty"`
	Platform    *DNSPlatformSpecApplyConfiguration `json:"platform,omitempty"`
}

// DNSSpecApplyConfiguration constructs an declarative configuration of the DNSSpec type for use with
// apply.
func DNSSpec() *DNSSpecApplyConfiguration {
	return &DNSSpecApplyConfiguration{}
}

// WithBaseDomain sets the BaseDomain field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BaseDomain field is set to the value of the last call.
func (b *DNSSpecApplyConfiguration) WithBaseDomain(value string) *DNSSpecApplyConfiguration {
	b.BaseDomain = &value
	return b
}

// WithPublicZone sets the PublicZone field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PublicZone field is set to the value of the last call.
func (b *DNSSpecApplyConfiguration) WithPublicZone(value *DNSZoneApplyConfiguration) *DNSSpecApplyConfiguration {
	b.PublicZone = value
	return b
}

// WithPrivateZone sets the PrivateZone field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PrivateZone field is set to the value of the last call.
func (b *DNSSpecApplyConfiguration) WithPrivateZone(value *DNSZoneApplyConfiguration) *DNSSpecApplyConfiguration {
	b.PrivateZone = value
	return b
}

// WithPlatform sets the Platform field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Platform field is set to the value of the last call.
func (b *DNSSpecApplyConfiguration) WithPlatform(value *DNSPlatformSpecApplyConfiguration) *DNSSpecApplyConfiguration {
	b.Platform = value
	return b
}