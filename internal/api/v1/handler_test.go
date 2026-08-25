package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/gin-gonic/gin"
)

func TestValidateDateOffsetBoundaries(t *testing.T) {
	for _, offset := range []int{0, 180} {
		if err := validateDateOffset(offset); err != nil {
			t.Fatalf("date_offset=%d 应被接受: %v", offset, err)
		}
	}
	for _, offset := range []int{-1, 181} {
		if err := validateDateOffset(offset); err == nil {
			t.Fatalf("date_offset=%d 应被拒绝", offset)
		}
	}
}

func TestRunClassroomQueryRejectsDateOffsetOutsideAllowedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	for _, offset := range []int{-1, 181} {
		_, err := handler.runClassroomQuery(ctx, model.QueryRequest{
			BuildingName: "老文史楼",
			StartNode:    "01",
			EndNode:      "02",
			DateOffset:   offset,
		})
		if err == nil {
			t.Fatalf("date_offset=%d 未被拒绝", offset)
		}
		if _, ok := err.(errBadRequest); !ok {
			t.Fatalf("date_offset=%d 错误类型=%T，期望 errBadRequest", offset, err)
		}
	}
}

func TestQueryFullDayStatusRejectsDateOffsetOutsideAllowedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/query-full-day",
		bytes.NewBufferString(`{"building":"老文史楼","date_offset":181}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewHandler(nil, nil).QueryFullDayStatus(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("HTTP 状态码错误: got=%d want=%d body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
