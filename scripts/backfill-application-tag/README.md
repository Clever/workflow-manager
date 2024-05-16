# backfill-application-tag

This is a script for adding the tag `application` to existing state machines that have a `workflow-definition-name` tag, copying the value over. It was written because (a) we like the `application` tag at Clever for cost attribution (used widely, not just with `workflow-manager`, and (b) a bug in workflow-manager caused this tag to not be applied.

To run, use `go run .` in this directory. Requires AWS permissions. There are several constants at the top of that file that control its behavior, see comments in code.
