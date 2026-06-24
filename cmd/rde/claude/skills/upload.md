## upload — copy a file or directory from the user's machine into the session

When the user asks to bring one of their local files into this session — "upload
my cert from ~/Downloads", "send my local config over", "add this file from my
machine" — call upload. The file lives on the user's machine (you can't see it
here); this copies it from there into a folder in this session.

```bash
curl -sS -X POST "$URL/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"localPath": "~/Downloads/cert.p12", "remoteFolder": "/absolute/folder/in/this/session"}'
```

- `localPath` (required): the path **on the user's machine** to copy. The user
  has to name it (you can't list their local files). A relative path is taken
  relative to the project directory they launched from; `~` means their home.
- `remoteFolder` (required): the destination folder **here in the session** to
  copy into. Use an absolute path — run `pwd` to get your current directory if the
  user just means "here".

On success the response is `{"uploaded": true, "remoteFolder": "…"}`. Tell the
user where it landed in the session. If the local file doesn't exist the response
explains that — relay it so they can correct the path.
