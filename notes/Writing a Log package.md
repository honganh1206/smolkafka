# Writing a Log package

Logs - sometimes called write-ahead logs, transaction logs, or commitlogs - are crucial for storage engines, message queues, version control, etc.

## The log is a powerful tool

Logs are used to improve data integrity.

For example: The `ext` filesystems log changes to a journal instead of directly changing the disk's data file. When the filesystem has safely written the changes to the journal, it applies the changes to the data files (Write-ahead logs).

Databases can use logs for state restoration: We take a snapshot of a database, then we can replay the logs until it's at the point of time we want.

## How logs work

A log is an **append-only** sequence of records.

When we append a record to a log, _the log assigns the record a unique and sequential offset_ that acts as the ID for that record.

Think of the log like a table: It always orders the records by time and indexes each record by offset and time created.

We cannot append to the same file forever, so _we split the log into a list of segments_. We delete old segments when the log grows too big, taking up disk space (We can use background process to do this).

We actively write to the **active segment**. When we fill an active segmnet, we create new segment and make it the active segment.

### Segments

Each segment comprises a store file and an index file. The store file stores the record data (We append to this), while the index file is where we index each record in the store file.

## Indexing

The index file keeps track of entries added to the log stream. It comprises a persisted file and a memory-mapped file.

Reads and writes to the index file all go through the in-memory map.

The service learns the next record's offset by _looking at the last entry of the index_ (reading the last 12 bytes of the file)
