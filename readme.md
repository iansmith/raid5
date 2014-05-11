* set your GOPATH to something sensible, like /tmp/iansmith
* set your PATH to something sensible derived from GOPATH, like /tmp/iansmith/bin

* go get -u github.com/iansmith/raid5/ws

* run the webserver with something like `/tmp/iansmith/bin/ws`
* it will dump out the key directories that you may want to look in

* in another shell
* try uploading a file with curl: `curl -i -H "Content-type:text/plain" -XPUT --data-binary @/etc/services http://localhost:8080/raid5/services`

* look at the log messages in the webserver, you'll see it writing files telling you what it did
* try deleting one of the copies you just created, like this (alter the temp directory): `rm  rm /var/folders/mh/lz5hcg3j5s16vh5ycgqgw254k2gwp1/T/raid5705775459/services`

* try getting the file back via CURL like this `curl -H -i  http://localhost:8080/raid5/services > /tmp/services`
* compare to prove that even after deleting the underlying storage you didn't lose anything
* diff /tmp/services /etc/services
* you may see some curl-crufties in that file, but the content is the same

* try running the tests with
* go test -v raid5