package firehose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/CharlesDardaman/blueskyfirehose/diskutil"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/sequential"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
	"github.com/bluesky-social/indigo/util/cliutil"
	"github.com/bluesky-social/indigo/xrpc"
	logging "github.com/ipfs/go-log"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("firehose")

var authFile = "bsky.auth"

var Firehose = &cli.Command{
	Name: "firehose",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "authed", //if you want to be authed or not.
		},
		&cli.Int64Flag{
			Name:  "mf", //min follower count to print
			Value: 0,
		},
		&cli.BoolFlag{
			Name: "likes", //if you want likes to show or not
		},
		&cli.BoolFlag{
			Name: "save",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
		defer stop()

		if !diskutil.FileExists(authFile) && cctx.Bool("authed") {
			//create session and write it to disk

			if cctx.Args().Len() < 2 {
				return fmt.Errorf("please provide username and password")
			}

			sess, err := createSession(cctx)
			if err != nil {
				return err
			}

			// Saves the bsky.auth file
			err = diskutil.WriteStructToDisk(sess, authFile)
			if err != nil {
				return err
			}

		}

		arg := "wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos"

		//Set if empty
		if cctx.String("pds-host") == "" {
			cctx.Set("pds-host", "https://bsky.social")
		}
		var xrpcc *xrpc.Client
		var err error
		if cctx.Bool("authed") {
			cctx.Set("auth", authFile)
			xrpcc, err = cliutil.GetXrpcClient(cctx, true)
			if err != nil {
				return err
			}
		}

		fmt.Println("dialing: ", arg)
		d := websocket.DefaultDialer
		con, _, err := d.Dial(arg, http.Header{})
		if err != nil {
			return fmt.Errorf("dial failure: %w", err)
		}

		fmt.Println("Stream Started", time.Now().Format(time.RFC3339))
		defer func() {
			fmt.Println("Stream Exited", time.Now().Format(time.RFC3339))
		}()

		go func() {
			<-ctx.Done()
			_ = con.Close()
		}()

		// HandleRepoStream is the main beef of this function
		// It will run on each event and switch on the event type and run the callbacks passed to it in
		// events.RepoStreamCallbacks

		rscb := &events.RepoStreamCallbacks{
			RepoCommit: func(evt *comatproto.SyncSubscribeRepos_Commit) error {

				// Returns a... readrepo?
				rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
				if err != nil {
					fmt.Println(err)
				} else {

					for _, op := range evt.Ops {
						ek := repomgr.EventKind(op.Action)
						switch ek {
						case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
							//fmt.Println("got record", op.Path, op.Cid, op.Action, evt.Seq, evt.Repo)
							rc, rec, err := rr.GetRecord(ctx, op.Path)
							if err != nil {
								e := fmt.Errorf("getting record %s (%s) within seq %d for %s: %w", op.Path, *op.Cid, evt.Seq, evt.Repo, err)
								log.Error(e)
								return nil
							}

							if lexutil.LexLink(rc) != *op.Cid {
								return fmt.Errorf("mismatch in record and op cid: %s != %s", rc, *op.Cid)
							}

							//fmt.Println("got record", rc, rec)
							banana := lexutil.LexiconTypeDecoder{
								Val: rec,
							}

							var pst = appbsky.FeedPost{}
							b, err := banana.MarshalJSON()
							if err != nil {
								fmt.Println(err)
							}

							//fmt.Println(string(b))

							err = json.Unmarshal(b, &pst)
							if err != nil {
								fmt.Println(err)
							}

							var userProfile *appbsky.ActorDefs_ProfileViewDetailed
							var replyUserProfile *appbsky.ActorDefs_ProfileViewDetailed
							if cctx.Bool("authed") {
								userProfile, err = appbsky.ActorGetProfile(context.TODO(), xrpcc, evt.Repo)
								if err != nil {
									fmt.Println(err)

									//try a refresh
									sess, err := refreshSession(cctx)
									if err == nil {
										err = diskutil.WriteStructToDisk(sess, authFile)
										if err != nil {
											return err
										}
										//reset xrpcc
										xrpcc, err = cliutil.GetXrpcClient(cctx, true)
										if err != nil {
											return err
										}

									}

								}
								if pst.Reply != nil {
									replyUserProfile, err = appbsky.ActorGetProfile(context.TODO(), xrpcc, strings.Split(pst.Reply.Parent.Uri, "/")[2])
									if err != nil {
										fmt.Println(err)
									}
								}

							}

							//Handle if its a post
							if pst.LexiconTypeID == "app.bsky.feed.post" {

								PrintPost(cctx, pst, userProfile, replyUserProfile, nil, op.Path)

							} else if pst.LexiconTypeID == "app.bsky.feed.like" && cctx.Bool("likes") {

								// fmt.Println("Like")
								// fmt.Println(string(b))

								var like = appbsky.FeedLike{}
								err = json.Unmarshal(b, &like)
								if err != nil {
									fmt.Println(err)
								}
								// fmt.Println(like)
								// fmt.Println(like.Subject)

								// fmt.Println(like.Subject.Uri[strings.LastIndex(like.Subject.Uri[:strings.LastIndex(like.Subject.Uri, "/")], "/")+1:])

								// fmt.Println(like.Subject.Uri[strings.LastIndex(like.Subject.Uri, "/")+1:])

								likedDid := strings.Split(like.Subject.Uri, "/")[2]

								rrb, err := comatproto.SyncGetRepo(ctx, xrpcc, likedDid, "")
								if err != nil {
									fmt.Println(err)
									continue
								}

								rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(rrb))
								if err != nil {
									fmt.Println(err)
									continue
								}

								_, rec, err := rr.GetRecord(ctx, like.Subject.Uri[strings.LastIndex(like.Subject.Uri[:strings.LastIndex(like.Subject.Uri, "/")], "/")+1:])
								if err != nil {
									log.Error(err)
									return nil
								}

								//fmt.Println("got record", rc, rec)
								banana := lexutil.LexiconTypeDecoder{
									Val: rec,
								}

								var pst = appbsky.FeedPost{}
								b, err := banana.MarshalJSON()
								if err != nil {
									fmt.Println(err)
								}

								err = json.Unmarshal(b, &pst)
								if err != nil {
									fmt.Println(err)
								}

								likedUserProfile, err := appbsky.ActorGetProfile(context.TODO(), xrpcc, likedDid)
								if err != nil {
									fmt.Println(err)
								}

								PrintPost(cctx, pst, likedUserProfile, nil, userProfile, like.Subject.Uri[strings.LastIndex(like.Subject.Uri, "/")+1:])

							}

						case repomgr.EvtKindDeleteRecord:
							// if err := cb(ek, evt.Seq, op.Path, evt.Repo, nil, nil); err != nil {
							// 	return err
							// }
						}
					}

				}

				return nil
			},
			RepoHandle: func(handle *comatproto.SyncSubscribeRepos_Handle) error {
				b, err := json.Marshal(handle)
				if err != nil {
					return err
				}
				fmt.Println("RepoHandle")
				fmt.Println(string(b))
				return nil
			},
			RepoInfo: func(info *comatproto.SyncSubscribeRepos_Info) error {

				b, err := json.Marshal(info)
				if err != nil {
					return err
				}
				fmt.Println("RepoInfo")
				fmt.Println(string(b))

				// } else {
				// 	fmt.Printf("INFO: %s: %v\n", info.Name, info.Message)
				// }

				return nil
			},
			RepoMigrate: func(mig *comatproto.SyncSubscribeRepos_Migrate) error {
				b, err := json.Marshal(mig)
				if err != nil {
					return err
				}
				fmt.Println("RepoMigrate")
				fmt.Println(string(b))
				return nil
			},
			RepoTombstone: func(tomb *comatproto.SyncSubscribeRepos_Tombstone) error {
				b, err := json.Marshal(tomb)
				if err != nil {
					return err
				}
				fmt.Println("RepoTombstone")
				fmt.Println(string(b))
				return nil
			},
			LabelLabels: func(labels *comatproto.LabelSubscribeLabels_Labels) error {
				b, err := json.Marshal(labels)
				if err != nil {
					return err
				}
				fmt.Println("LabelLabels")
				fmt.Println(string(b))
				return nil
			},
			LabelInfo: func(info *comatproto.LabelSubscribeLabels_Info) error {
				b, err := json.Marshal(info)
				if err != nil {
					return err
				}
				fmt.Println("LabelInfo")
				fmt.Println(string(b))
				return nil
			},

			Error: func(errf *events.ErrorFrame) error {
				return fmt.Errorf("error frame: %s: %s", errf.Error, errf.Message)
			},
		}

		seqScheduler := sequential.NewScheduler(con.RemoteAddr().String(), rscb.EventHandler)
		return events.HandleRepoStream(ctx, con, seqScheduler)
	},
}

func createSession(cctx *cli.Context) ([]byte, error) {
	xrpcc, err := cliutil.GetXrpcClient(cctx, false)
	if err != nil {
		return nil, err
	}
	handle := cctx.Args().Get(0)
	password := cctx.Args().Get(1)

	ses, err := comatproto.ServerCreateSession(context.TODO(), xrpcc, &comatproto.ServerCreateSession_Input{
		Identifier: handle,
		Password:   password,
	})
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(ses, "", "  ")
}

func refreshSession(cctx *cli.Context) ([]byte, error) {
	xrpcc, err := cliutil.GetXrpcClient(cctx, true)
	if err != nil {
		return nil, err
	}

	a := xrpcc.Auth
	a.AccessJwt = a.RefreshJwt

	ctx := context.TODO()
	nauth, err := comatproto.ServerRefreshSession(ctx, xrpcc)
	if err != nil {
		return nil, err
	}

	return json.Marshal(nauth)

}

func PrintPost(cctx *cli.Context, pst appbsky.FeedPost, userProfile, replyUserProfile, likingUserProfile *appbsky.ActorDefs_ProfileViewDetailed, postPath string) {
	if userProfile != nil && userProfile.FollowersCount != nil {

		//Try to use the display name and follower count if we can get it

		var enoughfollowers bool
		if *userProfile.FollowersCount >= int64(cctx.Int("mf")) {
			enoughfollowers = true
		}
		if likingUserProfile != nil {
			if *likingUserProfile.FollowersCount >= int64(cctx.Int("mf")) {
				enoughfollowers = true
			}

		}

		if enoughfollowers {

			var rply, likedTxt string
			if pst.Reply != nil && replyUserProfile != nil && replyUserProfile.FollowersCount != nil {
				rply = " ➡️ " + replyUserProfile.Handle + ":" + strconv.Itoa(int(*userProfile.FollowersCount)) + "\n" //+ "https://staging.bsky.app/profile/" + strings.Split(pst.Reply.Parent.Uri, "/")[2] + "/post/" + path.Base(pst.Reply.Parent.Uri) + "\n"
			} else if likingUserProfile != nil {
				likedTxt = likingUserProfile.Handle + ":" + strconv.Itoa(int(*likingUserProfile.FollowersCount)) + " ❤️ "
				rply = ":\n"
			} else {
				rply = ":\n"
			}

			url := "https://bsky.app/profile/" + userProfile.Handle + "/post/" + path.Base(postPath)
			fmtdstring := likedTxt + userProfile.Handle + ":" + strconv.Itoa(int(*userProfile.FollowersCount)) + rply + pst.Text + "\n" + url + "\n"
			fmt.Println(fmtdstring)
			if cctx.Bool("save") {
				go diskutil.SavePostToDisk(fmtdstring)
			}
		}
	} else {
		fmt.Println(pst.Text)
		if cctx.Bool("save") {
			go diskutil.SavePostToDisk(pst.Text)
		}
	}
}
