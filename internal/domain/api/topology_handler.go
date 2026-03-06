package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/topology"
	apiv1 "github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1"
)

type topologyHandler struct{}

func NewTopologyHandler() *topologyHandler {
	return &topologyHandler{}
}

func (h *topologyHandler) ValidateTopology(ctx context.Context, req *connect.Request[apiv1.ValidateTopologyRequest]) (*connect.Response[apiv1.ValidateTopologyResponse], error) {
	suite := req.Msg.GetTestSuite()
	if suite == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("test_suite is required"))
	}

	var validationErrors []*apiv1.ValidationError

	for i, test := range suite.GetTests() {
		tmpl := test.GetDatabaseTemplate()
		if tmpl == nil {
			if test.GetConnectionString() == "" {
				validationErrors = append(validationErrors, &apiv1.ValidationError{
					FieldPath: fmt.Sprintf("tests[%d].database_ref", i),
					Severity:  apiv1.ValidationSeverity_VALIDATION_SEVERITY_ERROR,
					Message:   "database_template or connection_string is required",
				})
			}
			continue
		}

		if err := topology.ValidateDatabaseTemplate(ctx, tmpl); err != nil {
			validationErrors = append(validationErrors, &apiv1.ValidationError{
				FieldPath: fmt.Sprintf("tests[%d].database_template", i),
				Severity:  apiv1.ValidationSeverity_VALIDATION_SEVERITY_ERROR,
				Message:   err.Error(),
			})
		}
	}

	return connect.NewResponse(&apiv1.ValidateTopologyResponse{
		Errors: validationErrors,
	}), nil
}
