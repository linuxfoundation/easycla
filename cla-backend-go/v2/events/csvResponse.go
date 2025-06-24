// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
)

// CSVEventsResponse creates a new response handler for CSV files
func CSVEventsResponse(filename string, events *models.EventList) middleware.Responder {
	return &CSVEventsResponderFunc{
		Filename: filename,
		Events:   events,
	}
}

// CSVEventsResponderFunc wraps a func as a Responder interface
type CSVEventsResponderFunc struct {
	Filename string
	Events   *models.EventList
}

// WriteResponse writes to the response
func (fn CSVEventsResponderFunc) WriteResponse(rw http.ResponseWriter, pr runtime.Producer) {
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", fn.Filename))
	rw.Header().Set(runtime.HeaderContentType, runtime.CSVMime)
	if err := pr.Produce(rw, fn.convertToCSV(fn.Events, ",")); err != nil {
		msg := fmt.Sprintf("issue converting event models to CSV format - error: %+v.", err)
		log.Warn(msg)
		rw.Header().Del("Content-Disposition")
		rw.Header().Set(runtime.HeaderContentType, runtime.JSONMime)
		rw.WriteHeader(http.StatusInternalServerError)
		_, writeErr := rw.Write([]byte("{\"error\":\"" + msg + "\"}"))
		if writeErr != nil {
			log.Warnf("issue writing error response response - error: %+v", writeErr)
		}
		return
	}
}

func (fn CSVEventsResponderFunc) convertToCSV(events *models.EventList, delimiter string) []string {
	var csvLines []string
	var b bytes.Buffer
	b.WriteString("Date-Time,Activity,Performed By,Company Name,Project")
	csvLines = append(csvLines, b.String())
	b.Reset()
	for _, event := range events.Events {
		et, err := utils.ParseDateTime(event.EventTime)
		if err != nil {
			b.WriteString(event.EventTime)
		} else {
			b.WriteString(utils.TimeToString(et))
		}
		b.WriteString(delimiter)
		b.WriteString(`"`)
		b.WriteString(event.EventData)
		b.WriteString(`"`)
		b.WriteString(delimiter)
		b.WriteString(`"`)
		b.WriteString(event.UserName)
		b.WriteString(`"`)
		b.WriteString(delimiter)
		b.WriteString(`"`)
		b.WriteString(event.EventCompanyName)
		b.WriteString(`"`)
		b.WriteString(delimiter)
		b.WriteString(`"`)
		b.WriteString(event.EventProjectSFName)
		b.WriteString(`"`)
		csvLines = append(csvLines, b.String())
		b.Reset()
	}
	return csvLines
}
