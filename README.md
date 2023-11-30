# protoc-gen-go-rsn
Generation of helper components which represent resource names, following the guidance in [AIP-4231](https://google.aip.dev/client-libraries/4231).
The goal is to make it easier for users to piece resource names together.
See [example.rsn_test.go](example/examplersn/example.rsn_test.go) for an example on using the generated code.

⚠️ This project is still in POC phase: it's not feature complete (according to AIP-4231), has no testing etc.
   You should use it with careful consideration.

To generate the example generated code, perform the following steps:

1. Clone [the googleapis repository](https://github.com/googleapis/googleapis) and set the environment variable `$GOOGLEAPIS` to the cloned directory.
2. Execute the following script:
```bash
go build -o protoc-gen-go-rsn .
cd example
protoc -I . -I "$GOOGLEAPIS" \
  --plugin=../protoc-gen-go-rsn \
  --go-rsn_out=. \
  --go-rsn_opt=module=github.com/simonbos/protoc-gen-go-rsn/example \
  --go-rsn_opt="Mexample.proto=github.com/simonbos/protoc-gen-go-rsn/example/examplersn;examplersn" \
  example.proto
```
