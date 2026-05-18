package httphandlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	mocks "github.com/poyrazk/thecloud/internal/core/ports/mocks"
	"github.com/poyrazk/thecloud/pkg/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testZoneName = "example.com"
	zonesPath    = "/dns/zones"
	testIPAddr   = "1.1.1.1"
	recordsPath  = "/dns/records"
)

func TestCreateZoneHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		r := gin.New()
		r.POST(zonesPath, handler.CreateZone)

		reqBody := map[string]interface{}{
			"name":        testZoneName,
			"description": "Test Zone",
			"vpc_id":      uuid.New().String(),
		}
		body, _ := json.Marshal(reqBody)

		zone := &domain.DNSZone{ID: uuid.New(), Name: testZoneName}
		svc.On("CreateZone", mock.Anything, mock.Anything, testZoneName, "Test Zone").Return(zone, nil)

		req, _ := http.NewRequest(http.MethodPost, zonesPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp httputil.Response
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		dataJSON, _ := json.Marshal(resp.Data)
		var response domain.DNSZone
		err = json.Unmarshal(dataJSON, &response)
		require.NoError(t, err)
		assert.Equal(t, testZoneName, response.Name)
	})

	t.Run("invalid input", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		r := gin.New()
		r.POST(zonesPath, handler.CreateZone)

		testCases := []struct {
			name    string
			payload map[string]interface{}
			code    int
			msg     string
		}{
			{
				name:    "missing name",
				payload: map[string]interface{}{"vpc_id": uuid.New().String()},
				code:    http.StatusBadRequest,
				msg:     "field 'name' is required",
			},
			{
				name:    "missing vpc_id",
				payload: map[string]interface{}{"name": testZoneName},
				code:    http.StatusBadRequest,
				msg:     "field 'vpc_id' is required",
			},
			{
				name:    "invalid vpc_id format",
				payload: map[string]interface{}{"name": testZoneName, "vpc_id": "not-a-valid-uuid"},
				code:    http.StatusBadRequest,
				msg:     "field 'vpc_id' must be a valid UUID",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest(http.MethodPost, zonesPath, bytes.NewBuffer(body))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Equal(t, tc.code, w.Code)
				assert.Contains(t, w.Body.String(), tc.msg)
			})
		}
	})
}

func TestGetZoneHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		zoneID := uuid.New()
		r := gin.New()
		r.GET("/dns/zones/:id", handler.GetZone)

		zone := &domain.DNSZone{ID: zoneID, Name: testZoneName}
		svc.On("GetZone", mock.Anything, zoneID.String()).Return(zone, nil)

		req, _ := http.NewRequest(http.MethodGet, zonesPath+"/"+zoneID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestDeleteZoneHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		zoneID := uuid.New()
		r := gin.New()
		r.DELETE(zonesPath+"/:id", handler.DeleteZone)

		svc.On("DeleteZone", mock.Anything, zoneID.String()).Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, zonesPath+"/"+zoneID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestListZonesHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		r := gin.New()
		r.GET(zonesPath, handler.ListZones)

		zones := []*domain.DNSZone{{ID: uuid.New(), Name: "zone1.com"}}
		svc.On("ListZones", mock.Anything).Return(zones, nil)

		req, _ := http.NewRequest(http.MethodGet, zonesPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp httputil.Response
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		dataJSON, _ := json.Marshal(resp.Data)
		var response []*domain.DNSZone
		err = json.Unmarshal(dataJSON, &response)
		require.NoError(t, err)
		assert.Len(t, response, 1)
	})
}

func TestCreateRecordHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		zoneID := uuid.New()
		r := gin.New()
		r.POST("/dns/zones/:id/records", handler.CreateRecord)

		reqBody := map[string]interface{}{
			"name":    "www",
			"type":    "A",
			"content": testIPAddr,
			"ttl":     3600,
		}
		body, _ := json.Marshal(reqBody)

		record := &domain.DNSRecord{ID: uuid.New(), Name: "www", Type: domain.RecordTypeA}
		svc.On("CreateRecord", mock.Anything, zoneID, "www", domain.RecordTypeA, testIPAddr, 3600, mock.Anything).Return(record, nil)

		req, _ := http.NewRequest(http.MethodPost, zonesPath+"/"+zoneID.String()+"/records", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestListRecordsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		zoneID := uuid.New()
		r := gin.New()
		r.GET(zonesPath+"/:id/records", handler.ListRecords)

		records := []*domain.DNSRecord{{ID: uuid.New(), Name: "www", Type: domain.RecordTypeA}}
		svc.On("ListRecords", mock.Anything, zoneID).Return(records, nil)

		req, _ := http.NewRequest(http.MethodGet, zonesPath+"/"+zoneID.String()+"/records", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetRecordHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		recordID := uuid.New()
		r := gin.New()
		r.GET(recordsPath+"/:id", handler.GetRecord)

		record := &domain.DNSRecord{ID: recordID, Name: "www", Type: domain.RecordTypeA}
		svc.On("GetRecord", mock.Anything, recordID).Return(record, nil)

		req, _ := http.NewRequest(http.MethodGet, recordsPath+"/"+recordID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestUpdateRecordHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		recordID := uuid.New()
		r := gin.New()
		r.PUT(recordsPath+"/:id", handler.UpdateRecord)

		reqBody := map[string]interface{}{
			"content": testIPAddr,
			"ttl":     3600,
		}
		body, _ := json.Marshal(reqBody)

		record := &domain.DNSRecord{ID: recordID, Content: testIPAddr}
		svc.On("UpdateRecord", mock.Anything, recordID, testIPAddr, 3600, mock.Anything).Return(record, nil)

		req, _ := http.NewRequest(http.MethodPut, recordsPath+"/"+recordID.String(), bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestDeleteRecordHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := mocks.NewDNSService(t)
		handler := NewDNSHandler(svc)
		recordID := uuid.New()
		r := gin.New()
		r.DELETE(recordsPath+"/:id", handler.DeleteRecord)

		svc.On("DeleteRecord", mock.Anything, recordID).Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, recordsPath+"/"+recordID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
