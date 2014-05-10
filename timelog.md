### Timelog

* 07:43 Start, setup enable script and dirs
* 07:44 Read about raid-5 on wikipedia
* 07:48 Started on task, decided to use a test struct "dirs" to simplify testing
* 07:51 Thinking about using Writer instead of os.File
* 08:04 Wrote test dir cleanup, unnecessary in /tmp?
* 08:04 checksums make it easier to test, looking up how to get md5 in go
* 08:19 Lookup "saturday night special" in wikipedia because wondering if Lynard Skynard was pro gun control
* 08:19 simplifying first test to get something passing
* 08:32 back from bathroom break and more coffee
* 08:32 decided that filenames (strings) should only be messed with once
* 08:48 Woot! first test passing
* 08:51 decided to always write exactly block-size bytes and then handle dealing with file size at higher level
* 08:59 just realized I'm doing the assigment test first, why is it that this "just happens" in Go, but is huge effort in Ruby?
* 09:02 more coffee!
* 09:09 wondering if there is any efficient way to do this where you write alternating bytes into d1 and d2--seems unlikely
* 09:35 first bug caught by test (although it was so serious it would not have persisted, wrong block written into part2)
* 09:40 found second bug: BLOCK_SIZE =0xffff and HALF_BLOCK=BLOCK_SIZE>>2 is awkward because the first and second blocks are different sizes (odd/even)
* 09:42, assume BLOCK_SIZE is even and add a sanity check
* 09:52 slowed down to ingest bagel, fixed test code bug
* 10:10 the padding with zeros part is definitely silly stupid
* 10:10 realized need a content test for the padding with zeros
* 10:25 found a third stupid bug (< instead of > )
* half hour break for shower, take dog out, start laundry
* 10:55 realized in shower that need to store the length of the files in the name (easier than putting it in the content)