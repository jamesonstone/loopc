.DEFAULT_GOAL := help

KIT ?= kit

.PHONY: help check

help:
	@printf '%s\n' 'Project developer workflow'
	@printf '%s\n' ''
	@printf '%s\n' '  check   validate the project document and instruction contract'

check:
	$(KIT) check --project
