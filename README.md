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
    
```sh
go run . firehose --mf 100 --likes # show posts and likes by users with a minimum of 100 followers
```
    
## Without Authentication
    
```sh
go run . firehose
```

## With Authentication
If it's the first time you need to enter your credentials. An App Password can be created at https://bsky.app/settings/app-passwords

```sh
go run . firehose -authed someusername.bsky.social superpassword
```
    
If you have a custom domain, use the email address you used to sign up Bluesky instead. 
    


After you've already run it once, it will save your credentials in a file called .bsky.auth in your home directory.

```sh
go run . firehose -authed
```

# Acknowledgements

[Bluesky Social Github](https://github.com/bluesky-social/)


