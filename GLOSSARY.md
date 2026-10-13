# mangarr

mangarr downloads manga chapters from scanlation and aggregator sources into local archives, either once on request or continuously for monitored series.

## Language

### Monitoring

**Monitored series**:
A series that the operator configured for unattended monitoring, from one source.
_Avoid_: Monitored manga (in prose), watched manga, tracked series

**Check**:
One pass of the monitor over all monitored series.
_Avoid_: Cycle, tick, run

**Check interval**:
The time between two checks.
_Avoid_: Time frame, polling period

**New chapter**:
A chapter that a source lists for a monitored series with a number above the highest chapter seen on the last successful check of that series.
_Avoid_: Latest chapter (one check can find several new chapters)

### Library

**Archive**:
The `.cbz` file that holds the pages of one downloaded chapter.
_Avoid_: Download, file, CBZ
