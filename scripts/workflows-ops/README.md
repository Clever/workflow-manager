# Workflow Manager Operations Script

This script is used to perform simple operations like cancel and refresh on all workflows for a given workflow name and status.

## Usage

The script accepts the following command-line arguments:

- `workflow`: The name of the workflow, for example `multiverse:master`.
- `cmd`: The command to execute. Can be set to `cancel` or `refresh`.
- `status`: The status of the workflow to filter by, for example `queued` or `running`.
- `rate-limit`: A cap on the number of the workflows to enqueue for refresh per second.

Example usage:

```bash
go run main.go -workflow multiverse:master -status running -cmd refresh
```

## Notes

- You should be careful when running this script since the impact of running this script with wrong inputs is quite high! Consider asking for another set of eyes when you run this script.
- If you use the default `rate-limit`, you may experience some DynamoDB throttling, especially at the beginning before the table scales up. You can start off with a value of 10 or so, and ramp up to
  potentially as much as 100 or 125 as the table scales up. Note the DDB capacity also depends on the workflow - more DDB capacity will be used for large workflows with many steps.
- If you want to refresh _all_ workflows, you can do the following:
  - Change to `ark-config`, do a `git pull`, then run Run `ls -1 | rev | cut -c 5- | rev > workflows.txt`.
  - Move that `workflows.txt` file here.
  - Adjust the rate limit if desired in `./run-all.sh`
  - Do `./run-all.sh`
