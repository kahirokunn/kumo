package sqs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestService_DispatchAction_QueueTags(t *testing.T) {
	t.Parallel()

	svc := &Service{
		storage: NewMemoryStorage("http://localhost:4566"),
		baseURL: "http://localhost:4566",
	}

	call := func(target string, payload any) *httptest.ResponseRecorder {
		t.Helper()

		var body []byte
		if payload != nil {
			var err error
			body, err = json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
		}

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("X-Amz-Target", "AmazonSQS."+target)
		req.Header.Set("Content-Type", "application/x-amz-json-1.0")

		rec := httptest.NewRecorder()
		svc.DispatchAction(rec, req)
		return rec
	}

	createResp := call("CreateQueue", map[string]any{
		"QueueName": "ack-tags",
		"tags": map[string]string{
			"key1": "val1",
		},
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateQueue status = %d, want %d", createResp.Code, http.StatusOK)
	}

	tagsResp := call("ListQueueTags", map[string]any{
		"QueueUrl": "http://localhost:4566/000000000000/ack-tags",
	})
	if tagsResp.Code != http.StatusOK {
		t.Fatalf("ListQueueTags status = %d, want %d", tagsResp.Code, http.StatusOK)
	}

	var listResp ListQueueTagsResponse
	if err := json.Unmarshal(tagsResp.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(listResp.Tags) != 1 || listResp.Tags["key1"] != "val1" {
		t.Fatalf("unexpected ListQueueTags response: %#v", listResp.Tags)
	}

	tagResp := call("TagQueue", map[string]any{
		"QueueUrl": "http://kumo:4566/000000000000/ack-tags",
		"Tags": map[string]string{
			"key2": "val2",
		},
	})
	if tagResp.Code != http.StatusOK {
		t.Fatalf("TagQueue status = %d, want %d", tagResp.Code, http.StatusOK)
	}

	untagResp := call("UntagQueue", map[string]any{
		"QueueUrl": "http://kumo:4566/000000000000/ack-tags",
		"TagKeys": []string{
			"key1",
		},
	})
	if untagResp.Code != http.StatusOK {
		t.Fatalf("UntagQueue status = %d, want %d", untagResp.Code, http.StatusOK)
	}

	finalResp := call("ListQueueTags", map[string]any{
		"QueueUrl": "http://localhost:4566/000000000000/ack-tags",
	})
	if finalResp.Code != http.StatusOK {
		t.Fatalf("final ListQueueTags status = %d, want %d", finalResp.Code, http.StatusOK)
	}

	listResp = ListQueueTagsResponse{}
	if err := json.Unmarshal(finalResp.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(listResp.Tags) != 1 || listResp.Tags["key2"] != "val2" {
		t.Fatalf("unexpected tags after update: %#v", listResp.Tags)
	}
}
