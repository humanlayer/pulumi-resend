.PHONY: provider schema codegen clean

provider:
	mkdir -p bin
	go build -o bin/pulumi-resource-resend ./provider/cmd/pulumi-resource-resend

schema: provider
	pulumi package get-schema ./bin/pulumi-resource-resend | jq 'del(.version)' > provider/cmd/pulumi-resource-resend/schema.json

codegen: schema
	pulumi package gen-sdk ./bin/pulumi-resource-resend --language nodejs --out sdk

clean:
	rm -rf bin
