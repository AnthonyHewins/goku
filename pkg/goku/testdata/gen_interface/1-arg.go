package goku

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/airspace-link-inc/asl-lib/geojsonpb"
	asloperationsvc "github.com/airspace-link-inc/sauron/gen/go/asloperationsvc/v1"
	"github.com/airspace-link-inc/sauron/gen/go/providersvc/v1"
	"github.com/airspace-link-inc/sauron/internal/api"
	"github.com/airspace-link-inc/sauron/internal/dbase"
	providerClients "github.com/airspace-link-inc/sauron/internal/eye/provider_clients"
	"github.com/airspace-link-inc/sauron/internal/model"
	"github.com/airspace-link-inc/sauron/internal/producer"
	"github.com/airspace-link-inc/sauron/pkg/sauron"
	"github.com/airspace-link-inc/sauron/protomap"
	"github.com/google/uuid"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	otelCodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FilterParams struct {
	DateRangeStart, DateRangeEnd time.Time
	IDs                          []string
	FlightTypes                  []asloperationsvc.FlightType
	PilotIDS                     []string
	AircraftIDs                  []string
	OwnerIDs                     []string
	Name                         string
	Desc                         string
	Keyword                      string
}

//go:generate goku iface Controller -m MockT -n Interface -o 1-expected.txt
type Controller struct {
	tracer         trace.Tracer
	logger         *slog.Logger
	reader, writer *sql.DB
	queries        dbase.Querier
	natsProducer   producer.Producer
	clients        *providerClients.ProviderClients
}

func NewController(traceName string, logger *slog.Logger, reader, writer *sql.DB, natsProducer producer.Producer, clients *providerClients.ProviderClients) *Controller {
	return &Controller{
		tracer:       otel.Tracer(traceName),
		logger:       logger,
		reader:       reader,
		writer:       writer,
		queries:      dbase.New(),
		natsProducer: natsProducer,
		clients:      clients,
	}
}

func datastoreToPBF(x dbase.GetOperationRow) (*asloperationsvc.Operation, error) {
	geom, err := geojsonpb.PBFGeom(x.Geog)
	if err != nil {
		return nil, err
	}

	var aircraftId *string
	if x.AircraftID.Valid {
		uuid := x.AircraftID.UUID.String()
		aircraftId = &uuid
	}

	providers := make([]providersvc.ApprovalProvider, len(x.Providers))
	for i, v := range x.Providers {
		providers[i] = sauron.FromProvider(v)
	}

	return &asloperationsvc.Operation{
		Id:                     x.ID.String(),
		PilotId:                x.Owner,
		Name:                   x.Name,
		CreatedBy:              x.CreatedBy,
		CreatedDate:            timestamppb.New(x.CreatedDate),
		FlightType:             protomap.ToFlightType(x.FlightType),
		Boundary:               geom,
		StartTime:              timestamppb.New(x.StartTime),
		EndTime:                timestamppb.New(x.EndTime),
		AircraftId:             aircraftId,
		AuthorizationProviders: providers,
		MinAltitude:            float64(x.MinAltitude),
		MaxAltitude:            float64(x.MaxAltitude),
		AltitudeUnit:           protomap.ToOperationAltitudeUnitSvc(x.AltitudeUnit),
		AltitudeReference:      protomap.ToOperationAltitudeReferenceSvc(x.AltitudeReference),
		Timezone:               x.Timezone,
		Desc:                   x.Desc.String,
	}, nil
}

type MockProducer struct{}

func (m *MockProducer) SendASLOperation(eventType string, id string, payload dbase.GetOperationRow) error {
	return nil
}

func TestCreateOperation_Resolve(t *testing.T) {
	tests := []struct {
		name                string
		createOperationFunc func() error
		Operation           model.Operation
		expectedErr         error
	}{
		{
			name: "Successful operation",
			createOperationFunc: func() error {
				return nil
			},
			Operation: model.Operation{
				ID:                uuid.New(),
				Owner:             "Test",
				Name:              "Operation Test",
				CreatedBy:         "Test",
				MinAltitude:       0.0,
				MaxAltitude:       100.0,
				AltitudeUnit:      sauron.AltitudeUnitFeet,
				AltitudeReference: sauron.AltitudeReferenceAgl,
				StartTime:         time.Now().Add(time.Minute * 5),
				EndTime:           time.Now().Add(time.Minute * 5),
				AircraftID:        uuid.NullUUID{UUID: uuid.New(), Valid: true},
				Timezone:          "America/New_York",
				Boundary:          testutil.PolygonBahamas.AsGeometry(),
				FlightType:        sauron.FlightTypeCommercial,
			},
			expectedErr: nil,
		},
		{
			name: "Validation error",
			createOperationFunc: func() error {
				return nil
			},
			Operation: model.Operation{
				ID:                uuid.New(),
				Owner:             "Test",
				CreatedBy:         "Test",
				MinAltitude:       0.0,
				MaxAltitude:       100.0,
				AltitudeUnit:      sauron.AltitudeUnitFeet,
				AltitudeReference: sauron.AltitudeReferenceAgl,
				StartTime:         time.Now(),
				EndTime:           time.Now(),
				AircraftID:        uuid.NullUUID{UUID: uuid.New(), Valid: true},
				Timezone:          "America/New_York",
				Boundary:          testutil.PolygonBahamas.AsGeometry(),
				FlightType:        sauron.FlightTypeCommercial,
			},
			expectedErr: errors.New("operation is missing a name"),
		},
		{
			name: "CreateOperation error",
			createOperationFunc: func() error {
				return errors.New("db error")
			},
			Operation: model.Operation{
				ID:                uuid.MustParse("2891903d-a5ca-4938-a2e6-384846af0ac4"),
				Owner:             "Test",
				Name:              "Operation Test",
				CreatedBy:         "Test",
				MinAltitude:       0.0,
				MaxAltitude:       100.0,
				AltitudeUnit:      sauron.AltitudeUnitFeet,
				AltitudeReference: sauron.AltitudeReferenceAgl,
				StartTime:         time.Now().Add(time.Minute * 5),
				EndTime:           time.Now().Add(time.Minute * 6),
				AircraftID:        uuid.NullUUID{UUID: uuid.New(), Valid: true},
				Timezone:          "America/New_York",
				Boundary:          testutil.PolygonBahamas.AsGeometry(),
				FlightType:        sauron.FlightTypeCommercial,
			},
			expectedErr: api.FromErr(errors.New("db error"), codes.Internal, "failed upserting operation %s: db error", "2891903d-a5ca-4938-a2e6-384846af0ac4"),
		},
	}
	for _, tt := range tests {
		qt := dbmock.QuerierTest{
			CreateOperationFunc: tt.createOperationFunc,
			GetOperationFunc:    func() (dbase.GetOperationRow, error) { return dbase.GetOperationRow{}, nil },
		}
		p := &CreateOperation{
			logger:       slog.New(slog.DiscardHandler),
			queries:      qt,
			natsProducer: &MockProducer{},
			Operation:    tt.Operation,
		}
		t.Run(tt.name, func(t *testing.T) {
			err := p.Resolve(context.Background())
			if tt.expectedErr != nil {
				assert.EqualError(t, tt.expectedErr, err.Error())
				return
			}
			assert.Nil(t, err)
		})
	}
}

type CreateOperation struct {
	logger       *slog.Logger
	reader       *sql.DB
	writer       *sql.DB
	queries      dbase.Querier
	natsProducer producer.Producer

	model.Operation
}

func (c *Controller) NewCreateOperation(op model.Operation) *CreateOperation {
	return &CreateOperation{
		logger:       c.logger,
		reader:       c.reader,
		writer:       c.writer,
		queries:      dbase.New(),
		natsProducer: c.natsProducer,
		Operation:    op,
	}
}

func (p *CreateOperation) Resolve(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	err := p.queries.CreateOperation(ctx, p.writer, p.ToCreateOperationParams())
	if err != nil {
		p.logger.ErrorContext(ctx, "failed upsert", "err", err)
		return api.FromErr(err, codes.Internal, "failed upserting operation %s: %v", p.ID, err)
	}

	operation, err := p.queries.GetOperation(ctx, p.reader, p.Operation.ID)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed retrieving operation", "err", err)
		// The Operation was created we should probably not error out. Just log. Possible post on another queue about the error to try to resend it?
	}
	err = p.natsProducer.SendASLOperation("update", p.Operation.ID.String(), operation)
	if err != nil {
		p.logger.ErrorContext(ctx, "send operation to message bus", "error", err)
		// The Operation was created we should probably not error out. Just log. Possible post on another queue about the error to try to resend it?
	}

	return nil
}

func (p *CreateOperation) ToCreateOperationParams() dbase.CreateOperationParams {
	return dbase.CreateOperationParams{
		ID:                p.Operation.ID,
		Owner:             p.Operation.Owner,
		OrgID:             sql.NullString{String: p.Operation.OrgID, Valid: p.Operation.OrgID != ""},
		Name:              p.Operation.Name,
		Archived:          p.Operation.Archived,
		CreatedBy:         p.Operation.CreatedBy,
		MinAltitude:       float32(p.Operation.MinAltitude),
		MaxAltitude:       float32(p.Operation.MaxAltitude),
		AltitudeUnit:      p.Operation.AltitudeUnit,
		AltitudeReference: p.Operation.AltitudeReference,
		StartTime:         p.Operation.StartTime,
		EndTime:           p.Operation.EndTime,
		AircraftID:        p.Operation.AircraftID,
		Timezone:          p.Operation.Timezone,
		StGeomfromwkb:     p.Operation.Boundary,
		FlightType:        p.Operation.FlightType,
		Desc:              sql.NullString{String: p.Operation.Desc, Valid: p.Operation.Desc != ""},
		WebhookUrl:        sql.NullString{String: p.Operation.CallbackUrl, Valid: p.Operation.CallbackUrl != ""},
		ClientID:          sql.NullString{String: p.Operation.ClientID, Valid: p.Operation.ClientID != ""},
	}
}

type DeleteOperationArgs struct {
	ID      uuid.UUID
	OwnerID string
	OrgID   string
}

func (d *DeleteOperationArgs) Validate() error {
	if d.ID == uuid.Nil {
		return api.NewErr(codes.InvalidArgument, "missing ID")
	}

	if d.OwnerID == "" {
		return api.NewErr(codes.InvalidArgument, "pilot ID missing")
	}

	return nil
}

type ChildClient interface {
	Provider() sauron.Provider
	Delete(ctx context.Context, id uuid.UUID) error
}

type DeleteOperation struct {
	tracer         trace.Tracer
	logger         *slog.Logger
	reader, writer *sql.DB
	queries        dbase.Querier
	clients        *providerClients.ProviderClients
	natsProducer   producer.Producer

	DeleteOperationArgs
}

// NewDeleteOperation creates a delete operation. After creation, you need to supply an Org ID
// or a Pilot ID
func (c *Controller) NewDeleteOperation(id uuid.UUID, ownerID string) *DeleteOperation {
	return &DeleteOperation{
		tracer:              c.tracer,
		logger:              c.logger,
		writer:              c.writer,
		reader:              c.reader,
		queries:             dbase.New(),
		clients:             c.clients,
		natsProducer:        c.natsProducer,
		DeleteOperationArgs: DeleteOperationArgs{ID: id, OwnerID: ownerID},
	}
}

func (p *DeleteOperation) WithOrgID(orgID string) *DeleteOperation {
	p.OrgID = orgID
	return p
}

func (p *DeleteOperation) Resolve(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	ctx, span := p.tracer.Start(ctx, "deleting operation "+p.ID.String())
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	args := p.DeleteOperationArgs
	l := p.logger.With("trace-id", span.SpanContext().TraceID(), "args", args)

	operation, err := p.queries.GetOperation(ctx, p.reader, p.ID)
	if err != nil {
		l.ErrorContext(ctx, "failed retrieving operation", "err", err)
		return api.FromErr(err, codes.NotFound, "operation %s does not exist", p.ID)
	}

	providers, err := p.queries.GetApprovalProviders(ctx, p.reader, args.ID)
	if err != nil {
		msg := "failed to retrieve approval providers"
		l.ErrorContext(ctx, msg, "err", err)
		return status.Error(codes.Internal, msg)
	}

	if err = p.clients.Fanout(ctx, &providerClients.FanoutArgs{
		Providers: providers,
		Fn:        providerClients.Delete,
		ID:        p.ID,
		Owner:     p.OwnerID,
		Org:       p.OrgID,
	}); err != nil {
		return err
	}

	_, err = p.queries.DeleteOperation(ctx, p.writer, dbase.DeleteOperationParams{
		ID:    args.ID,
		Owner: args.OwnerID,
		OrgID: sql.NullString{String: args.OrgID, Valid: args.OrgID != ""},
	})

	if errors.Is(err, sql.ErrNoRows) {
		l.ErrorContext(ctx, "operation not found", "err", err)
		return api.FromErr(err, codes.NotFound, "operation %s does not exist, or you don't have access to it", args.ID)
	}

	if err != nil {
		l.ErrorContext(ctx, "failed deleting operation", "err", err)
		return api.FromErr(err, codes.Internal, "failed deleting operation %s: %v", args.ID, err)
	}

	err = p.natsProducer.SendASLOperation("DELETE", p.ID.String(), operation)
	if err != nil {
		p.logger.ErrorContext(ctx, "send operation to message bus", "error", err)
		// The Operation was deleted, we should probably not error out. Just log. Possible post on another queue about the error to try to resend it?
	}

	p.logger.Debug("operation deleted successfully", "operation_id", args.ID)

	return nil
}

type GetOperationArgs struct {
	ID      uuid.UUID
	OwnerID string
	OrgID   string
}

func (g *GetOperationArgs) Validate() error {
	if g.ID == uuid.Nil {
		return status.Errorf(codes.InvalidArgument, "operation ID is the null UUID, which is invalid")
	}

	return nil
}

type GetOperation struct {
	tracer  trace.Tracer
	logger  *slog.Logger
	reader  *sql.DB
	queries dbase.Querier

	GetOperationArgs
}

func (c *Controller) NewGetOperationByPilot(id uuid.UUID, ownerID string) *GetOperation {
	return &GetOperation{
		tracer:           c.tracer,
		logger:           c.logger,
		reader:           c.reader,
		queries:          dbase.New(),
		GetOperationArgs: GetOperationArgs{ID: id, OwnerID: ownerID},
	}
}

func (p *GetOperation) WithOrgID(orgID string) *GetOperation {
	p.OrgID = orgID
	return p
}

//nolint:gocognit
func (p *GetOperation) Resolve(ctx context.Context) (*asloperationsvc.Operation, error) {
	args := p.GetOperationArgs
	ctx, span := p.tracer.Start(ctx, fmt.Sprintf("fetching operation %s", args.ID))
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	l := p.logger.With("trace-id", span.SpanContext().TraceID(), "req", args)

	span.AddEvent("Fetching from database")
	model, err := p.queries.GetOperation(ctx, p.reader, args.ID)

	if err != nil {
		l.ErrorContext(ctx, "failed fetching operation", "err", err)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, api.FromErr(err, codes.NotFound, "operation %s does not exist", p.ID)
		}

		return nil, api.FromErr(err, codes.Internal, "failed fetching operation: %v", err)
	}

	// if requested operation is an individual operation, block if requester is not the owner
	if model.OrgID.String == "" && args.OwnerID != model.Owner {
		return nil, status.Errorf(
			codes.PermissionDenied,
			"you do not have access to operation %s",
			p.ID,
		)
	}

	// if requested operation is part of an org, block if requester org doesn't match operation org
	if model.OrgID.String != "" && model.OrgID.String != args.OrgID {
		return nil, status.Errorf(
			codes.PermissionDenied,
			"you do not have access to operation %s",
			p.ID,
		)
	}

	polygon, ok := model.Geog.AsPolygon()
	if !ok {
		return nil, api.NewErr(codes.Internal, "only polygon geometries are currently supported, got %s", model.Geog.Type())
	}

	geom, err := geojsonpb.PBFPolygon(polygon)
	if err != nil {
		l.ErrorContext(ctx, "failed converting polygon to PBF", "err", err)
		return nil, err
	}

	var aircraftId *string
	if model.AircraftID.Valid {
		uuid := model.AircraftID.UUID.String()
		aircraftId = &uuid
	}

	providers := make([]providersvc.ApprovalProvider, len(model.Providers))
	for i, v := range model.Providers {
		providers[i] = sauron.FromProvider(v)
	}

	return &asloperationsvc.Operation{
		Id:                     model.ID.String(),
		PilotId:                model.Owner,
		Name:                   model.Name,
		CreatedBy:              model.CreatedBy,
		CreatedDate:            timestamppb.New(model.CreatedDate),
		FlightType:             protomap.ToFlightType(model.FlightType),
		Boundary:               geom,
		StartTime:              timestamppb.New(model.StartTime),
		EndTime:                timestamppb.New(model.EndTime),
		AircraftId:             aircraftId,
		AuthorizationProviders: providers,
		MinAltitude:            float64(model.MinAltitude),
		MaxAltitude:            float64(model.MaxAltitude),
		AltitudeUnit:           protomap.ToOperationAltitudeUnitSvc(model.AltitudeUnit),
		AltitudeReference:      protomap.ToOperationAltitudeReferenceSvc(model.AltitudeReference),
		Timezone:               model.Timezone,
		Desc:                   model.Desc.String,
	}, nil
}

// Get an operation without performing any validation
func (c *Controller) GetAdmin(ctx context.Context, id uuid.UUID) (*asloperationsvc.Operation, error) {
	l := c.logger.With("id", id)

	if id == uuid.Nil {
		l.ErrorContext(ctx, "uuid is zero value")
		return nil, api.NewErr(codes.InvalidArgument, "can't fetch operation: uuid is zero")
	}

	model, err := c.queries.GetOperation(ctx, c.reader, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		l.ErrorContext(ctx, "operation not found", "error", err)
		return nil, api.FromErr(err, codes.NotFound, "operation %s not found", id)
	case err == nil:
		// no-op
	case errors.Is(err, context.DeadlineExceeded):
		l.ErrorContext(ctx, "database took too long responding, deadline exceeded", "error", err)
		return nil, api.FromErr(err, codes.DeadlineExceeded, "transaction took too long, deadline exceeded fetching %s", id)
	default:
		l.ErrorContext(ctx, "failed fetching operation", "error", err)
		return nil, api.FromErr(err, codes.Internal, "failed fetching %s", id)
	}

	polygon, ok := model.Geog.AsPolygon()
	if !ok {
		return nil, api.NewErr(codes.Internal, "only polygon geometries are currently supported, got %s", model.Geog.Type())
	}

	geom, err := geojsonpb.PBFPolygon(polygon)
	if err != nil {
		l.ErrorContext(ctx, "failed converting polygon to PBF", "err", err)
		return nil, api.FromErr(err, codes.Internal, "polygon is invalid: %s", err)
	}

	var aircraftId *string
	if model.AircraftID.Valid {
		uuid := model.AircraftID.UUID.String()
		aircraftId = &uuid
	}

	providers := make([]providersvc.ApprovalProvider, len(model.Providers))
	for i, v := range model.Providers {
		providers[i] = sauron.FromProvider(v)
	}

	return &asloperationsvc.Operation{
		Id:                     model.ID.String(),
		PilotId:                model.Owner,
		Name:                   model.Name,
		CreatedBy:              model.CreatedBy,
		CreatedDate:            timestamppb.New(model.CreatedDate),
		FlightType:             protomap.ToFlightType(model.FlightType),
		Boundary:               geom,
		StartTime:              timestamppb.New(model.StartTime),
		EndTime:                timestamppb.New(model.EndTime),
		AircraftId:             aircraftId,
		AuthorizationProviders: providers,
		MinAltitude:            float64(model.MinAltitude),
		MaxAltitude:            float64(model.MaxAltitude),
		AltitudeUnit:           protomap.ToOperationAltitudeUnitSvc(model.AltitudeUnit),
		AltitudeReference:      protomap.ToOperationAltitudeReferenceSvc(model.AltitudeReference),
		Timezone:               model.Timezone,
		Desc:                   model.Desc.String,
	}, nil
}

type ListOperationsByOrg struct {
	tracer  trace.Tracer
	logger  *slog.Logger
	reader  *sql.DB
	queries dbase.Querier

	Pilot, Org string
	filter     FilterParams
}

func (c *Controller) NewListOperationsByOrg(pilot, org string, filter FilterParams) *ListOperationsByOrg {
	return &ListOperationsByOrg{
		tracer:  c.tracer,
		logger:  c.logger,
		reader:  c.reader,
		queries: dbase.New(),
		Pilot:   pilot,
		Org:     org,
		filter:  filter,
	}
}

func (p *ListOperationsByOrg) Resolve(ctx context.Context) ([]*asloperationsvc.Operation, error) {
	if p.Org == "" {
		p.logger.ErrorContext(ctx, fmt.Sprintf("dev error: no org supplied to %T", p))
		return nil, status.Error(codes.InvalidArgument, "no org supplied")
	}

	ctx, span := p.tracer.Start(ctx, fmt.Sprintf("listing operations for %s", p.Org))
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	l := p.logger.With("trace-id", span.SpanContext().TraceID(), "req", p)

	//nolint:dupl
	params := dbase.ListOperationsParams{
		OrgId:          sql.NullString{String: p.Org, Valid: true}, // passing OrgId, so all org's operations will be selected
		DateRangeStart: sql.NullTime{Time: p.filter.DateRangeStart, Valid: !p.filter.DateRangeStart.IsZero()},
		DateRangeEnd:   sql.NullTime{Time: p.filter.DateRangeEnd, Valid: !p.filter.DateRangeEnd.IsZero()},
		Ids:            p.filter.IDs,
		FlightTypes:    protomap.FromFlightTypes(p.filter.FlightTypes),
		PilotIds:       p.filter.PilotIDS,
		AircraftIds:    p.filter.AircraftIDs,
		OwnerIds:       p.filter.OwnerIDs,
		Name:           sql.NullString{String: p.filter.Name, Valid: p.filter.Name != ""},
		Desc:           sql.NullString{String: p.filter.Desc, Valid: p.filter.Desc != ""},
		Keyword:        sql.NullString{String: p.filter.Keyword, Valid: p.filter.Keyword != ""},
	}

	models, err := p.queries.ListOperations(ctx, p.reader, params)
	if err != nil {
		l.ErrorContext(ctx, "failed query", "err", err)
		return nil, api.FromErr(err, codes.Internal, "failed fetching operations for %s: %v", p.Org, err)
	}

	operations := make([]*asloperationsvc.Operation, len(models))
	for i, model := range models {
		operations[i], err = datastoreToPBF(dbase.GetOperationRow(model))
		if err != nil {
			l.ErrorContext(ctx, "failed converting datastore object to pbf model", "error", err)
			return nil, status.Errorf(codes.Internal, "data is corrupted for operation %s", model.ID)
		}
	}

	return operations, nil
}

type ListOperationsByPilot struct {
	tracer  trace.Tracer
	logger  *slog.Logger
	reader  *sql.DB
	queries dbase.Querier

	owner  string
	filter FilterParams
}

func (c *Controller) NewListOperationsForPilot(owner string, filter FilterParams) *ListOperationsByPilot {
	return &ListOperationsByPilot{
		tracer:  c.tracer,
		logger:  c.logger,
		reader:  c.reader,
		queries: dbase.New(),
		owner:   owner,
		filter:  filter,
	}
}

func (p *ListOperationsByPilot) Resolve(ctx context.Context) ([]*asloperationsvc.Operation, error) {
	ctx, span := p.tracer.Start(ctx, fmt.Sprintf("listing operations for %s", p.owner))
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	l := p.logger.With("trace-id", span.SpanContext().TraceID(), "req", p)

	//nolint:dupl
	params := dbase.ListOperationsParams{
		Owner:          sql.NullString{String: p.owner, Valid: true}, // passing Owner, so only operations with this Owner and without OrgId will be selected
		DateRangeStart: sql.NullTime{Time: p.filter.DateRangeStart, Valid: !p.filter.DateRangeStart.IsZero()},
		DateRangeEnd:   sql.NullTime{Time: p.filter.DateRangeEnd, Valid: !p.filter.DateRangeEnd.IsZero()},
		Ids:            p.filter.IDs,
		FlightTypes:    protomap.FromFlightTypes(p.filter.FlightTypes),
		PilotIds:       p.filter.PilotIDS,
		AircraftIds:    p.filter.AircraftIDs,
		OwnerIds:       p.filter.OwnerIDs,
		Name:           sql.NullString{String: p.filter.Name, Valid: p.filter.Name != ""},
		Desc:           sql.NullString{String: p.filter.Desc, Valid: p.filter.Desc != ""},
		Keyword:        sql.NullString{String: p.filter.Keyword, Valid: p.filter.Keyword != ""},
	}

	models, err := p.queries.ListOperations(ctx, p.reader, params)
	if err != nil {
		l.ErrorContext(
			ctx, "failed query",
			"error", err,
		)

		return nil, api.FromErr(err, codes.Internal, "failed fetching operations for %s: %v", p.owner, err)
	}

	operations := make([]*asloperationsvc.Operation, len(models))
	for i, model := range models {
		operations[i], err = datastoreToPBF(dbase.GetOperationRow(model))
		if err != nil {
			l.ErrorContext(ctx, "failed converting datastore object to pbf model", "error", err)
			return nil, status.Errorf(codes.Internal, "data is corrupted for operation %s", model.ID)
		}
	}

	return operations, nil
}

type QueryOperations struct {
	tracer  trace.Tracer
	logger  *slog.Logger
	reader  *sql.DB
	queries dbase.Querier

	bbox            geom.Geometry
	startTime       time.Time
	endTime         time.Time
	pilot           string
	organizationIDs []string
}

func (c *Controller) NewQueryOperations(
	bbox geom.Geometry,
	startTime time.Time,
	endTime time.Time,
	pilot string,
	organizationIDs []string,
) *QueryOperations {

	return &QueryOperations{
		tracer:          c.tracer,
		logger:          c.logger,
		reader:          c.reader,
		queries:         dbase.New(),
		bbox:            bbox,
		startTime:       startTime,
		endTime:         endTime,
		pilot:           pilot,
		organizationIDs: organizationIDs,
	}
}

func (p *QueryOperations) validate() error {
	if p.queries == nil {
		return api.NewErr(codes.Internal, "unable to query operations")
	}

	if p.bbox.IsEmpty() && p.pilot == "" && p.startTime.IsZero() &&
		p.endTime.IsZero() && len(p.organizationIDs) == 0 {
		return api.NewErr(
			codes.InvalidArgument,
			"must provide at least one query filter",
		)
	}

	return nil
}

func (p *QueryOperations) Resolve(ctx context.Context) (
	[]*asloperationsvc.Operation, error) {

	ctx, span := p.tracer.Start(ctx, "querying operations")
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	l := p.logger.With("trace-id", span.SpanContext().TraceID(), "request", p)

	err = p.validate()
	if err != nil {
		l.ErrorContext(
			ctx, "failed to validate query operations input",
			"error", err,
		)

		return nil, err
	}

	var boundary sql.NullString
	if !p.bbox.IsEmpty() {
		boundary = sql.NullString{
			Valid:  true,
			String: p.bbox.AsText(),
		}
	}

	models, err := p.queries.QueryOperations(
		ctx,
		p.reader,
		dbase.QueryOperationsParams{
			Owner: sql.NullString{
				String: p.pilot,
				Valid:  p.pilot != "",
			},
			StartTime: sql.NullTime{
				Time:  p.startTime,
				Valid: !p.startTime.IsZero(),
			},
			EndTime: sql.NullTime{
				Time:  p.endTime,
				Valid: !p.endTime.IsZero(),
			},
			Boundary:        boundary,
			OrganizationIds: p.organizationIDs,
		},
	)
	if err != nil {
		l.ErrorContext(
			ctx, "failed query",
			"error", err,
		)

		return nil, api.FromErr(
			err,
			codes.Internal,
			"failed to query operations: %v", err,
		)
	}

	operations := make([]*asloperationsvc.Operation, len(models))
	for i, model := range models {
		operations[i], err = datastoreToPBF(dbase.GetOperationRow(model))
		if err != nil {
			l.ErrorContext(
				ctx,
				"failed converting datastore object to pbf model",
				"error", err,
			)

			return nil, status.Errorf(
				codes.Internal,
				"data is corrupted for operation %s",
				model.ID,
			)
		}
	}

	return operations, nil
}

type UpdateOperation struct {
	tracer       trace.Tracer
	logger       *slog.Logger
	reader       *sql.DB
	writer       *sql.DB
	queries      dbase.Querier
	natsProducer producer.Producer
	clients      *providerClients.ProviderClients

	model.Operation
}

func (c *Controller) NewUpdateOperation(op model.Operation) *UpdateOperation {
	return &UpdateOperation{
		tracer:       c.tracer,
		logger:       c.logger,
		reader:       c.reader,
		writer:       c.writer,
		queries:      dbase.New(),
		natsProducer: c.natsProducer,
		clients:      c.clients,
		Operation:    op,
	}
}

//nolint:gocognit
func (p *UpdateOperation) Resolve(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	ctx, span := p.tracer.Start(ctx, fmt.Sprintf("update operation %s", p.ID))
	defer span.End()

	var err error
	defer func() {
		if err == nil {
			span.SetStatus(otelCodes.Ok, "success")
			return
		}

		span.SetStatus(otelCodes.Error, "failure")
		span.RecordError(err)
	}()

	l := p.logger.With("trace-id", span.SpanContext().TraceID())

	providers, err := p.queries.GetApprovalProviders(ctx, p.reader, p.ID)
	if err != nil {
		l.ErrorContext(ctx, "failed to get approval providers", "err", err)
		return api.FromErr(err, codes.Internal, "failed to get approval providers %s: %v", p.ID, err)
	}

	// This should skip the fanout because there are no approval providers if the operation is new
	if len(providers) != 0 {
		if err = p.clients.Fanout(ctx, &providerClients.FanoutArgs{
			Providers: providers,
			Fn:        providerClients.Update,
			ID:        p.ID,
			Owner:     p.Owner,
			Org:       p.OrgID,
		}); err != nil {
			return status.Error(codes.Internal, "one or more approvalProviders failed to update")
		}
	}

	rowsAffected, err := p.queries.UpdateOperation(ctx, p.writer, p.ToUpdateOperationParams())
	if err != nil {
		l.ErrorContext(ctx, "failed update", "error", err)
		return api.FromErr(err, codes.Internal, "failed update operation %s: %v", p.ID, err)
	}

	if affected, _ := rowsAffected.RowsAffected(); affected <= 0 {
		l.ErrorContext(ctx, "operation does not exist", "error", err)
		return api.FromErr(err, codes.NotFound, "operation %s does not exist", p.ID)
	}

	operation, err := p.queries.GetOperation(ctx, p.reader, p.Operation.ID)
	if err != nil {
		l.ErrorContext(ctx, "failed retrieving operation", "err", err)
		// The Operation was created we should probably not error out. Just log. Possible post on another queue about the error to try to resend it?
	}
	err = p.natsProducer.SendASLOperation("UPDATE", p.Operation.ID.String(), operation)
	if err != nil {
		l.ErrorContext(ctx, "send operation to message bus", "error", err)
		// The Operation was created we should probably not error out. Just log. Possible post on another queue about the error to try to resend it?
	}

	return nil
}

func (p *UpdateOperation) ToUpdateOperationParams() dbase.UpdateOperationParams {
	return dbase.UpdateOperationParams{
		ID:                p.Operation.ID,
		Owner:             p.Operation.Owner,
		OrgID:             sql.NullString{String: p.Operation.OrgID, Valid: p.Operation.OrgID != ""},
		Name:              p.Operation.Name,
		Archived:          p.Operation.Archived,
		MinAltitude:       float32(p.Operation.MinAltitude),
		MaxAltitude:       float32(p.Operation.MaxAltitude),
		AltitudeUnit:      p.Operation.AltitudeUnit,
		AltitudeReference: p.Operation.AltitudeReference,
		StartTime:         p.Operation.StartTime,
		EndTime:           p.Operation.EndTime,
		AircraftID:        p.Operation.AircraftID,
		Timezone:          p.Operation.Timezone,
		StGeomfromwkb:     p.Operation.Boundary,
		FlightType:        p.Operation.FlightType,
		Desc:              sql.NullString{String: p.Operation.Desc, Valid: p.Operation.Desc != ""},
	}
}
