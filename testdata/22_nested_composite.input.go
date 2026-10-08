package sample

func request() {
    req := DescribeItemsRequest{
        Context: common.Context{UserData: map[string]string{"request_id": "describe-request"}},
    }
    use(req)
}
