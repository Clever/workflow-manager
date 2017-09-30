.PHONY: vendor
vendor:
	rm -rf vendor
	mkdir -p vendor/github.com/Clever
	cd vendor/github.com/Clever && git clone git@github.com:Clever/workflow-manager.git old-workflow-manager
	cd vendor/github.com/Clever/old-workflow-manager && git ls-files | xargs sed -i 's/github.com\/Clever\/workflow-manager/github.com\/Clever\/old-workflow-manager/g'
	# convert attachWorkflowDefinition to a public method, since it's useful for migration
	cd vendor/github.com/Clever/old-workflow-manager && git ls-files | xargs sed -i 's/attachWorkflowDefinition/AttachWorkflowDefinition/g'
