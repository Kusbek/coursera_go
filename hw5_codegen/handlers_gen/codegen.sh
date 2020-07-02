#!/bin/bash

go build -o "codegen" codegen.go  && ./codegen api.go ../api_handlers.go