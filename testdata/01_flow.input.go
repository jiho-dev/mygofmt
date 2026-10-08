package sample
func schedule() error {
 req := request()
 result, err := call(req)
 if err != nil {
  return err
 }
 for _, host := range result.Hosts {
  assign(host)
 }
 response := result
 return response.Err
}
