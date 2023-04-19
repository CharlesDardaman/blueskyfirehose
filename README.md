# blueskyfirehose
Simple golang firehose for Bluesky.


# Usage

## Without Authentication

go run . firehose

## With Authentication
If it's the first time you need to enter your credentials.

go run . firehose -auth someusername.bsky.social superpassword

After you've already run it once, it will save your credentials in a file called .bsky.auth in your home directory.

go run . firehose -auth
```