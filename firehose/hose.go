package firehose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/CharlesDardaman/blueskyfirehose/diskutil"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	cliutil "github.com/bluesky-social/indigo/cmd/gosky/util"
	"github.com/bluesky-social/indigo/events"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
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
			err = diskutil.WriteStructToDisk(sess, authFile)
			if err != nil {
				return err
			}

		}

		arg := "wss://bsky.social/xrpc/com.atproto.sync.subscribeRepos"

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

		jsonfmt := cctx.Bool("json")

		fmt.Println("Stream Started", time.Now().Format(time.RFC3339))
		defer func() {
			fmt.Println("Stream Exited", time.Now().Format(time.RFC3339))
		}()

		go func() {
			<-ctx.Done()
			_ = con.Close()
		}()

		return events.HandleRepoStream(ctx, con, &events.RepoStreamCallbacks{
			RepoCommit: func(evt *comatproto.SyncSubscribeRepos_Commit) error {

				rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
				if err != nil {
					fmt.Println(err)
				} else {

					for _, op := range evt.Ops {
						ek := repomgr.EventKind(op.Action)
						switch ek {
						case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
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
							//fmt.Println(banana.Val)

							var pst = appbsky.FeedPost{}
							b, err := banana.MarshalJSON()
							if err != nil {
								fmt.Println(err)
							}

							err = json.Unmarshal(b, &pst)
							if err != nil {
								fmt.Println(err)
							}
							//Handle if its a post
							if pst.LexiconTypeID == "app.bsky.feed.post" {

								var userProfile *appbsky.ActorDefs_ProfileViewDetailed
								if cctx.Bool("authed") {
									userProfile, err = appbsky.ActorGetProfile(context.TODO(), xrpcc, evt.Repo)
									if err != nil {
										fmt.Println(err)
									}
								}

								//Try to use the display name and follower count if we can get it
								if userProfile != nil && userProfile.DisplayName != nil && userProfile.FollowersCount != nil {
									fmt.Println(*userProfile.DisplayName + ":" + strconv.Itoa(int(*userProfile.FollowersCount)) + ":" + pst.Text)
								} else {
									fmt.Println(pst.Text)
								}
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
			RepoInfo: func(info *comatproto.SyncSubscribeRepos_Info) error {
				if jsonfmt {
					b, err := json.Marshal(info)
					if err != nil {
						return err
					}
					fmt.Println(string(b))
				} else {
					fmt.Printf("INFO: %s: %v\n", info.Name, info.Message)
				}

				return nil
			},
			// TODO: all the other event types
			Error: func(errf *events.ErrorFrame) error {
				return fmt.Errorf("error frame: %s: %s", errf.Error, errf.Message)
			},
		})
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
