package wpapi

import (
  "github.com/kolo/xmlrpc"
  "net/http"
  "bytes"
  "io/ioutil"
)

type Client struct {
  Username  string  `json:"username"`
  Password  string  `json:"password"`
  BlogID    string  `json:"blog_id"`
  APIURL    string  `json:"apiurl"`
}

type Post struct {
  Header  string  `xmlrpc:"post_title"`
  Content string  `xmlrpc:"post_content"`
  ID      string  `xmlrpc:"post_id"`
}

func doHTTPPost(url, mime string, reqBytes []byte) ([]byte, error) {
  reader := bytes.NewReader(reqBytes)
  httpResp, err := http.Post(url, mime, reader)
  defer httpResp.Body.Close()
  if err != nil {
    return []byte{}, err
  }

  bytesResp, err := ioutil.ReadAll(httpResp.Body)
  return bytesResp, err
}

func (c *Client) GetPost(postID string) (Post, error) {
  reqBytes, err := xmlrpc.EncodeMethodCall("wp.getPost", c.BlogID, c.Username, c.Password,
    postID, []string{"post_title", "post_content", "post_id"})
  if err != nil {
    return Post{}, err
  }

  bytesResp, err := doHTTPPost(c.APIURL, "text/xml", reqBytes)
  if err != nil {
    return Post{}, err
  }

  post := Post{}
  xmlrpcResp := xmlrpc.NewResponse(bytesResp)
  err = xmlrpcResp.Unmarshal(&post)

  return post, err
}

func (c *Client) EditPost(post Post) (bool, error) {
  reqBytes, err := xmlrpc.EncodeMethodCall("wp.editPost", c.BlogID, c.Username, c.Password,
    post.ID, post)
  if err != nil {
    return false, err
  }

  bytesResp, err := doHTTPPost(c.APIURL, "text/xml", reqBytes)

  var ok bool = false

  xmlrpcResp := xmlrpc.NewResponse(bytesResp)
  err = xmlrpcResp.Unmarshal(&ok)
  if err != nil {
    return false, err
  }

  return ok, err
}

func (c *Client) GetAllPostIDs() ([]string, error) {
  reqBytes, err := xmlrpc.EncodeMethodCall("wp.getPosts", c.BlogID, c.Username, c.Password, struct{}{}, []string{"post_id"})

  if err != nil {
    return []string{}, err
  }

  bytesResp, err := doHTTPPost(c.APIURL, "text/xml", reqBytes)

  var wpResp []Post

  xmlrpcResp := xmlrpc.NewResponse(bytesResp)
  err = xmlrpcResp.Unmarshal(&wpResp)

  if err != nil {
    return []string{}, err
  }

  var result []string

  for _, p := range wpResp {
    result = append(result, p.ID)
  }

  return result, nil
}

func (c *Client) GetUsersBlog() (string, error) {
  reqBytes, err := xmlrpc.EncodeMethodCall("wp.getUsersBlogs", c.Username, c.Password)
  if err != nil {
    return "", err
  }

  bytesResp, err := doHTTPPost(c.APIURL, "text/xml", reqBytes)

  wpResp := []struct{
    BlogID  string  `xmlrpc:"blogid"`
  }{}

  xmlrpcResp := xmlrpc.NewResponse(bytesResp)
  err = xmlrpcResp.Unmarshal(&wpResp)
  if err != nil {
    return "", err
  }

  c.BlogID = wpResp[0].BlogID

  return wpResp[0].BlogID, nil
}
