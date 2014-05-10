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
* 11:04 got third test passing, added git for safety
* 11:12 wrote a half a content test in the wrong place, wasted time
* 11:34 going to try to encode the len and checksum in the name
* 11:57 after building the name encoding thing, I realized that the name
encoding of the hash means we have to change the name of the file *after
we have finished writing to it, ugh, ugly.
* 12:07 on a 32 bit machine, the len() is probably only 32 bits wide! 
this means we'd be limiting ourselves to 32 bit wide file sizes
* 12:23 because of the way we are doing naming, we want WriteAndClose()
to be a single logical operation
* 12:27 WriteAndClose() isn't really atomic but that fact isn't really
important in this simple example plus if there *is* a collision you
could actually get fancy because you know the file contents are the
same (due to hash)
* 12:41 now about to do the actual read with parity magic
* 12:50 don't want to be too aggressive about aborting if the directories get out of sync because that's actually a case we want to handle
* 12:55 zero size files end up NOT symlinking and computing a hash, which
makes sense but isn't regular
* 13:08 IMPROVEMENT NEEDED: Should make symlinks atomic in WriteAndClose
* 13:09 IMPROVEMENT NEEDED: Voting on errors in file creation?
* 13:10, Ok finally getting to actually doing reading
* 13:11 decided that open and read should be unified... then changed my mind because it makes the ReadFile method ugly
* 14:05, Massive bug found in the writing implementation
* 14:08 need to add a test for zero sized files specifically
* 14:18 wasn't a big bug, just an encoding issue (%x vs %d)
* 14:20 trying to deal with even/odd issues in the size of the buffer and
the offset makes me think I did something stupid
* 14:33 again, worrying that the len(foo) is an int not int64
* 15:07 took a break to hang with wife/dog
* 15:42 spent a bunch of time debugging something which boiled downed to "if you haven't written the code yet, it won't work"


