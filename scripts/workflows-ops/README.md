# Workflow Manager Operations Script

This script is used to perform simple operations like cancel and refresh on all workflows for a given workflow name and status.

## Usage

The script accepts the following command-line arguments:

- `workflow`: The name of the workflow, for example `multiverse:master`.
- `cmd`: The command to execute. Can be set to `cancel` or `refresh`.
- `status`: The status of the workflow to filter by, for example `queued` or `running`.

Example usage:

```bash
go run main.go -workflow multiverse:master -status running -cmd refresh
```

## Notes

You should be careful when running this script since the impact of running this script with wrong inputs is quite high! Consider asking for another set of eyes when you run this script.
