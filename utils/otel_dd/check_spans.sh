#!/bin/bash
curl -s http://localhost:8888/metrics | egrep 'otelcol_(receiver_accepted_spans|exporter_sent_spans|exporter_send_failed_spans)'
