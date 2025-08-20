go mod tidy
go build -o terraform-provider-infradots

if [ ! -d "~/.terraform.d/plugins/local/infradots/infradots/" ]; then
    mkdir -p ~/.terraform.d/plugins/local/infradots/infradots/0.0.1/linux_amd64
fi
mv terraform-provider-infradots ~/.terraform.d/plugins/local/infradots/infradots/0.0.1/linux_amd64/terraform-provider-infradots_v0.0.1
