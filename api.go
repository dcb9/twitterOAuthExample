package main

import (
	"errors"
	"fmt"
	"net/http"

	goTwitter "github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/gologin"
	oauth1Login "github.com/dghubble/gologin/oauth1"
	"github.com/dghubble/gologin/twitter"
	"github.com/dghubble/oauth1"
)

// Twitter login errors
var (
	ErrUnableToGetTwitterUser = errors.New("twitter: unable to get Twitter User")
)

func generateTwitterLoginURLHandler(config *oauth1.Config) http.Handler {
	fn := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		requestToken, _, err := oauth1Login.RequestTokenFromContext(ctx)
		if err != nil {
			fmt.Println(err)
			return
		}
		authorizationURL, err := config.AuthorizationURL(requestToken)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Fprint(w, authorizationURL.String())
	})
	return oauth1Login.LoginHandler(config, fn, nil)
}

func getTwitterUserInfo(config *oauth1.Config) http.Handler {
	fn := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		twitterUser, err := twitter.UserFromContext(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%#v", twitterUser)
	})
	return twitterCallbackHandler(config, fn, nil)
}

// twitterHandler is a http.Handler that gets the OAuth1 access token from
// the ctx and calls Twitter verify_credentials to get the corresponding User.
// If successful, the User is added to the ctx and the success handler is
// called. Otherwise, the failure handler is called.
func twitterHandler(config *oauth1.Config, success, failure http.Handler) http.Handler {
	if failure == nil {
		failure = gologin.DefaultFailureHandler
	}
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		accessToken, accessSecret, err := oauth1Login.AccessTokenFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		httpClient := config.Client(ctx, oauth1.NewToken(accessToken, accessSecret))
		twitterClient := goTwitter.NewClient(httpClient)
		accountVerifyParams := &goTwitter.AccountVerifyParams{
			IncludeEntities: goTwitter.Bool(false),
			SkipStatus:      goTwitter.Bool(true),
			IncludeEmail:    goTwitter.Bool(true),
		}
		user, resp, err := twitterClient.Accounts.VerifyCredentials(accountVerifyParams)
		err = validateResponse(user, resp, err)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		ctx = twitter.WithUser(ctx, user)
		success.ServeHTTP(w, req.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// twitterCallbackHandler handles Twitter callback requests by parsing the oauth token
// and verifier and adding the Twitter access token and User to the ctx. If
// authentication succeeds, handling delegates to the success handler,
// otherwise to the failure handler.
func twitterCallbackHandler(config *oauth1.Config, success, failure http.Handler) http.Handler {
	// oauth1.EmptyTempHandler -> oauth1.CallbackHandler -> TwitterHandler -> success
	success = twitterHandler(config, success, failure)
	success = oauth1Login.CallbackHandler(config, success, failure)
	return oauth1Login.EmptyTempHandler(success)
}

// validateResponse returns an error if the given Twitter user, raw
// http.Response, or error are unexpected. Returns nil if they are valid.
func validateResponse(user *goTwitter.User, resp *http.Response, err error) error {
	if err != nil || resp.StatusCode != http.StatusOK {
		return ErrUnableToGetTwitterUser
	}
	if user == nil || user.ID == 0 || user.IDStr == "" {
		return ErrUnableToGetTwitterUser
	}
	return nil
}
