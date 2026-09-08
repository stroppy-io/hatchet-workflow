package provider

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// All returns every provider.* schema, built fresh.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		AwsCredentials(),
		AwsSettings(),
		RegistryCredentials(),
		YandexCredentials(),
		YandexSettings(),
	}
}
