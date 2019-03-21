package force

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/nimajalali/go-force/sobjects"
)

func testForceApi(sfHandlerUrl string) *ForceApi {
	foauth := &forceOauth{
		AccessToken: "I am so an access token!",
		InstanceUrl: sfHandlerUrl,
	}
	forceAPI := &ForceApi{
		oauth:                  foauth,
		apiResources:           make(map[string]string),
		apiSObjects:            make(map[string]*SObjectMetaData),
		apiSObjectDescriptions: make(map[string]*SObjectDescription),
		apiVersion:             "v43.0",
	}
	forceAPI.SetAPISObjects(apiSObjects())
	forceAPI.SetAPIResources(apiResources())
	return forceAPI
}

func TestSObjCollectionCreate(t *testing.T) {
	t.Run("Formats requests", func(t *testing.T) {
		var requestBody string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Body.Close()
			requestBody = string(body)
			fmt.Fprintln(w, "Hello, client")
		}))
		defer ts.Close()

		forceAPI := testForceApi(ts.URL)

		expenses := SObjCollection{
			&MockSObject{
				Name: "object A",
				Cash: 32.9,
			},
			&MockSObject{
				Name: "object B",
				Cash: 98.34,
			},
		}

		forceAPI.SObjCollectionCreate(expenses, true)
		expectedRequestBody := `{
		"allOrNone" : true,
		"records" : [{
		   "attributes" : {"type" : "totally_valid_object__c"},
		   "name__c" : "object A",
		   "Cash_monaay__c" : 32.9
		}, {
		   "attributes" : {"type" : "totally_valid_object__c"},
		   "name__c" : "object B",
		   "Cash_monaay__c" : 98.34
		}]
	 }`
		if StripWhiteSpace(expectedRequestBody) != StripWhiteSpace(requestBody) {
			t.Errorf("expected actual request to be:\n%s, but got:\n%s", StripWhiteSpace(expectedRequestBody), StripWhiteSpace(requestBody))
		}
	})

	t.Run("Handles response", func(t *testing.T) {

		var testcases = []struct {
			name             string
			SFResponse       string
			expectedResponse *SObjCollectionResponse
			wantErr          bool
			expectedError    error
		}{
			{
				name: "all items processed succesfully",
				SFResponse: `[
					{
					   "id" : "001RM000003oLnnYAE",
					   "success" : true,
					   "errors" : [ ]
					},
					{
					   "id" : "003RM0000068xV6YAI",
					   "success" : true,
					   "errors" : [ ]
					}
				 ]`,
				wantErr: false,
				expectedResponse: &SObjCollectionResponse{
					SObjectResponse{
						Id:      "001RM000003oLnnYAE",
						Success: true,
					},
					SObjectResponse{
						Id:      "003RM0000068xV6YAI",
						Success: true,
					},
				},
			},
			{
				name: "some items cause errors, allOrNone false",
				SFResponse: `[
					{
					   "success" : false,
					   "errors" : [
						  {
							 "statusCode" : "DUPLICATES_DETECTED",
							 "message" : "Use one of these records?",
							 "fields" : [ ]
						  }
					   ]
					},
					{
					   "id" : "003RM0000068xVCYAY",
					   "success" : true,
					   "errors" : [ ]
					}
				 ]`,
				wantErr: false,
				expectedResponse: &SObjCollectionResponse{
					SObjectResponse{
						Success: false,
						Errors: []*ApiError{
							&ApiError{
								ErrorCode: "DUPLICATES_DETECTED",
								Message:   "Use one of these records?",
							},
						},
					},
					SObjectResponse{
						Id:      "003RM0000068xVCYAY",
						Success: true,
					},
				},
			},
			{
				name: "some items cause errors, allOrNone true",
				SFResponse: `[
					{
					   "success" : false,
					   "errors" : [
						  {
							 "statusCode" : "DUPLICATES_DETECTED",
							 "message" : "Use one of these records?",
							 "fields" : [ ]
						  }
					   ]
					},
					{
					   "success" : false,
					   "errors" : [
						  {
							 "statusCode" : "ALL_OR_NONE_OPERATION_ROLLED_BACK",
							 "message" : "Record rolled back because not all records were valid and the request was using AllOrNone header",
							 "fields" : [ ]
						  }
					   ]
					}
				]`,
				wantErr: false,
				expectedResponse: &SObjCollectionResponse{
					SObjectResponse{
						Success: false,
						Errors: []*ApiError{
							&ApiError{
								ErrorCode: "DUPLICATES_DETECTED",
								Message:   "Use one of these records?",
							},
						},
					},
					SObjectResponse{
						Success: false,
						Errors: []*ApiError{
							&ApiError{
								ErrorCode: "ALL_OR_NONE_OPERATION_ROLLED_BACK",
								Message:   "Record rolled back because not all records were valid and the request was using AllOrNone header",
							},
						},
					},
				},
			},
		}
		for _, testcase := range testcases {
			t.Run(testcase.name, func(t *testing.T) {

				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("content-type", "application/json")
					fmt.Fprint(w, testcase.SFResponse)
				}))
				defer ts.Close()

				forceAPI := testForceApi(ts.URL)

				expenses := SObjCollection{
					&MockSObject{
						Name: "object A",
						Cash: 32.9,
					},
					&MockSObject{
						Name: "object B",
						Cash: 98.34,
					},
				}

				resp, err := forceAPI.SObjCollectionCreate(expenses, true)
				if err != nil {
					t.Error("Didn't want an error, but got one:\n", err.Error())
				}
				if resp == nil {
					t.Errorf("Response was nil")
				}
				if len(*resp) < 1 {
					t.Error("expected response to contain an array")
				}
				if testcase.expectedResponse != resp {
					expResp := *testcase.expectedResponse
					for i, v := range *resp {
						expected := expResp[i]
						if v.Id != expected.Id {
							t.Errorf("Expected response:\n%#v item:%d to have ID:\n%v, but got:%v", *resp, i, expected.Id, v.Id)
						}
						if v.Errors != nil && expected.Errors == nil {
							t.Errorf("Did not expect response item to have an error, got:%#v", v.Errors)
						}
						if v.Errors != nil && expected.Errors != nil {
							compareErrors(t, v.Errors, expected.Errors)
							t.Errorf("Expected response:\n%#v item:%d to have ID:\n%v, but got:%v", *resp, i, expected.Id, v.Id)
						}
					}
				}
			})
		}
	})
}

func compareErrors(t *testing.T, actual, expected ApiErrors) {
	if len(actual) > len(expected) {
		t.Errorf("Actual errors length:(%d) > expected Errors length:(%d)", len(actual), len(expected))
	}
	if len(actual) < len(expected) {
		t.Errorf("Actual errors length:(%d) < expected Errors length:(%d)", len(actual), len(expected))
	}
	for i, v := range actual {
		exErr := expected[i]
		if v != exErr {
			t.Errorf("expected error:%#v, but got:%#v ", exErr, v)
		}
	}
}

func StripWhiteSpace(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}

type MockSObject struct {
	sobjects.BaseSObject
	Name string  `force:"name__c,omitempty"`
	Cash float32 `force:"Cash_monaay__c,omitempty"`
}

func (mso *MockSObject) ApiName() string {
	return "totally_valid_object__c"
}

//ExternalIdApiName to fulfill the SObject interface
func (mso *MockSObject) ExternalIdApiName() string {
	return "totally_valid_object__c"
}

func apiResources() io.Reader {
	return bytes.NewBufferString(`{
    "tooling": "/services/data/v43.0/tooling",
    "metadata": "/services/data/v43.0/metadata",
    "folders": "/services/data/v43.0/folders",
    "eclair": "/services/data/v43.0/eclair",
    "prechatForms": "/services/data/v43.0/prechatForms",
    "chatter": "/services/data/v43.0/chatter",
    "tabs": "/services/data/v43.0/tabs",
    "appMenu": "/services/data/v43.0/appMenu",
    "quickActions": "/services/data/v43.0/quickActions",
    "queryAll": "/services/data/v43.0/queryAll",
    "wave": "/services/data/v43.0/wave",
    "iot": "/services/data/v43.0/iot",
    "analytics": "/services/data/v43.0/analytics",
    "search": "/services/data/v43.0/search",
    "identity": "https://test.salesforce.com/id/00D2O0000008niwUAA/0052O000000XKDeQAO",
    "composite": "/services/data/v43.0/composite",
    "parameterizedSearch": "/services/data/v43.0/parameterizedSearch",
    "fingerprint": "/services/data/v43.0/fingerprint",
    "theme": "/services/data/v43.0/theme",
    "nouns": "/services/data/v43.0/nouns",
    "event": "/services/data/v43.0/event",
    "serviceTemplates": "/services/data/v43.0/serviceTemplates",
    "recent": "/services/data/v43.0/recent",
    "connect": "/services/data/v43.0/connect",
    "licensing": "/services/data/v43.0/licensing",
    "limits": "/services/data/v43.0/limits",
    "process": "/services/data/v43.0/process",
    "async-queries": "/services/data/v43.0/async-queries",
    "dedupe": "/services/data/v43.0/dedupe",
    "query": "/services/data/v43.0/query",
    "jobs": "/services/data/v43.0/jobs",
    "emailConnect": "/services/data/v43.0/emailConnect",
    "compactLayouts": "/services/data/v43.0/compactLayouts",
    "sobjects": "/services/data/v43.0/sobjects",
    "actions": "/services/data/v43.0/actions",
	"support": "/services/data/v43.0/support"
	}`)
}

func apiSObjects() io.Reader {
	return bytes.NewBufferString(`{
    "encoding": "UTF-8",
    "maxBatchSize": 200,
    "sobjects": [
        {
            "activateable": false,
            "createable": true,
            "custom": true,
            "customSetting": false,
            "deletable": true,
            "deprecatedAndHidden": false,
            "feedEnabled": true,
            "hasSubtypes": false,
            "isSubtype": false,
            "keyPrefix": "a5z",
            "label": "Totally Valid Object",
            "labelPlural": "Totally Valid Objects",
            "layoutable": true,
            "mergeable": false,
            "mruEnabled": true,
            "name": "totally_valid_object__c",
            "queryable": true,
            "replicateable": true,
            "retrieveable": true,
            "searchable": true,
            "triggerable": true,
            "undeletable": true,
            "updateable": true,
            "urls": {
                "compactLayouts": "/services/data/v43.0/sobjects/totally_valid_object__c/describe/compactLayouts",
                "rowTemplate": "/services/data/v43.0/sobjects/totally_valid_object__c/{ID}",
                "approvalLayouts": "/services/data/v43.0/sobjects/totally_valid_object__c/describe/approvalLayouts",
                "defaultValues": "/services/data/v43.0/sobjects/totally_valid_object__c/defaultValues?recordTypeId&fields",
                "describe": "/services/data/v43.0/sobjects/totally_valid_object__c/describe",
                "quickActions": "/services/data/v43.0/sobjects/totally_valid_object__c/quickActions",
                "layouts": "/services/data/v43.0/sobjects/totally_valid_object__c/describe/layouts",
                "sobject": "/services/data/v43.0/sobjects/totally_valid_object__c"
            }
        }
    ]}`)
}
