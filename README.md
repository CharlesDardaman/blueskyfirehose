# blueskyfirehose
Simple golang firehose for Bluesky.


# Usage

--mf <int> 
    Only show posts by users with a minimum follower count of <int>

--likes
    Show likes as well as posts

--help / -h 
    Show command help

Examples:
go run . firehose --mf 100 --likes # show posts and likes by users with a minimum of 100 followers

## Without Authentication

go run . firehose

## With Authentication
If it's the first time you need to enter your credentials.

go run . firehose -auth someusername.bsky.social superpassword

After you've already run it once, it will save your credentials in a file called .bsky.auth in your home directory.

go run . firehose -auth


# Acknowledgements

[Bluesky Social Github](https://github.com/bluesky-social/)


