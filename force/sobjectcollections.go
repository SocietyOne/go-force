package force

import (
	"path"
	"reflect"
)

//SObjCollectionRequest marshals to an SObject Collections Request
//https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_sobjects_collections_create.htm
type SObjCollectionRequest struct {
	AllOrNone bool      `force:"allOrNone,omitempty"`
	Records   []SObject `force:"records,omitempty"`
}

type SObjCollection []SObject

type SObjCollectionResponse []SObjectResponse

//SObjCollectionCreate creates a batch create request
func (forceApi *ForceApi) SObjCollectionCreate(in SObjCollection, allOrNone bool) (*SObjCollectionResponse, error) {
	req := &SObjCollectionRequest{
		AllOrNone: allOrNone,
		Records:   make([]SObject, len(in)),
	}
	for i, so := range in {
		v := reflect.ValueOf(so)
		bo := reflect.Indirect(v).FieldByName("BaseSObject").FieldByName("Attributes").FieldByName("Type")
		bo.SetString(so.ApiName())
		req.Records[i] = so
	}
	uri := path.Join(forceApi.apiResources[compositeKey], "sobjects")
	resp := &SObjCollectionResponse{}

	err := forceApi.Post(uri, nil, req, resp)
	return resp, err
}
