package tracing

import (
	"context"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gorm.io/gorm"
)

type key string

const (
	gormSpanStartTimeKey = key("dd-trace-go:span")
)

func Gorm(dbConn *gorm.DB) (err error) {
	cb := dbConn.Callback()

	afterFunc := func(operationName string) func(*gorm.DB) {
		return func(db *gorm.DB) {
			after(db, operationName)
		}
	}

	err = cb.Create().Before("gorm:create").Register("dd-trace-go:before_create", before)

	if err != nil {
		return
	}
	err = cb.Create().After("gorm:create").Register("dd-trace-go:after_create", afterFunc("gorm.create"))
	if err != nil {
		return
	}

	err = cb.Query().Before("gorm:query").Register("dd-trace-go:before_query", before)
	if err != nil {
		return
	}
	err = cb.Query().After("gorm:query").Register("dd-trace-go:after_query", afterFunc("gorm.query"))
	if err != nil {
		return
	}

	err = cb.Delete().Before("gorm:delete").Register("dd-trace-go:before_delete", before)
	if err != nil {
		return
	}
	err = cb.Delete().After("gorm:delete").Register("dd-trace-go:after_delete", afterFunc("gorm.delete"))
	if err != nil {
		return
	}

	err = cb.Raw().Before("gorm:raw").Register("dd-trace-go:before_raw", before)
	if err != nil {
		return
	}
	err = cb.Raw().After("gorm:raw").Register("dd-trace-go:after_raw", afterFunc("gorm.raw"))
	if err != nil {
		return
	}

	err = cb.Row().Before("gorm:row").Register("dd-trace-go:before_row", before)
	if err != nil {
		return
	}
	err = cb.Row().After("gorm:row").Register("dd-trace-go:after_row", afterFunc("gorm.row"))
	if err != nil {
		return
	}

	err = cb.Update().Before("gorm:Update").Register("dd-trace-go:before_update", before)
	if err != nil {
		return
	}
	err = cb.Update().After("gorm:Update").Register("dd-trace-go:after_update", afterFunc("gorm.update"))
	if err != nil {
		return
	}

	return
}

func before(scope *gorm.DB) {
	if scope.Statement != nil && scope.Statement.Context != nil {
		scope.Statement.Context = context.WithValue(scope.Statement.Context, gormSpanStartTimeKey, time.Now())
	}
}

func after(db *gorm.DB, operationName string) {
	if db.Statement == nil || db.Statement.Context == nil {
		return
	}

	ctx := db.Statement.Context
	t, ok := ctx.Value(gormSpanStartTimeKey).(time.Time)
	if !ok {
		return
	}

	opts := []ddtrace.StartSpanOption{
		tracer.StartTime(t),
		tracer.ServiceName(tracingParams.serviceName),
		tracer.SpanType(ext.DBSystemPostgreSQL),
		tracer.ResourceName(db.Statement.SQL.String()),
	}

	span, _ := tracer.StartSpanFromContext(ctx, operationName, opts...)

	span.Finish(tracer.WithError(db.Error))
}
