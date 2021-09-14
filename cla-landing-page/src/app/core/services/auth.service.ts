
import { Injectable } from '@angular/core';
import createAuth0Client from '@auth0/auth0-spa-js';
import Auth0Client from '@auth0/auth0-spa-js/dist/typings/Auth0Client';
import {
  from,
  of,
  Observable,
  BehaviorSubject,
  combineLatest,
  throwError,
} from 'rxjs';
import { tap, catchError, concatMap, shareReplay } from 'rxjs/operators';
import { Router } from '@angular/router';
import * as querystring from 'query-string';
import Url from 'url-parse';
import { EnvConfig } from '../../config/cla-env-utils';
import { AppSettings } from 'src/app/config/app-settings';
import { REDIRECT_AUTH_ROUTE } from 'src/app/config/auth-utils';
import { StorageService } from './storage.service';

@Injectable({
  providedIn: 'root',
})
export class AuthService {
  auth0Options = {
    domain: EnvConfig.default['auth0-domain'], // e.g linuxfoundation-dev.auth0.com
    clientId: EnvConfig.default['auth0-clientId'],
    useRefreshTokens: true
  };

  currentHref = window.location.href;

  loading$ = new BehaviorSubject<any>(true);
  // Create an observable of Auth0 instance of client
  auth0Client$ = (from(
    createAuth0Client({
      domain: this.auth0Options.domain,
      client_id: this.auth0Options.clientId,
      useRefreshTokens: this.auth0Options.useRefreshTokens
    })
  ) as Observable<Auth0Client>).pipe(
    shareReplay(1), // Every subscription receives the same shared value
    catchError((err) => {
      this.loading$.next(false);
      return throwError(err);
    })
  );
  // Define observables for SDK methods that return promises by default
  // For each Auth0 SDK method, first ensure the client instance is ready
  // concatMap: Using the client instance, call SDK method; SDK returns a promise
  // from: Convert that resulting promise into an observable
  isAuthenticated$ = this.auth0Client$.pipe(
    concatMap((client: Auth0Client) => from(client.isAuthenticated())),
    tap((res: any) => {
      // *info: once isAuthenticated$ responses , SSO sessiong is loaded
      this.loading$.next(false);
      this.loggedIn = res;
    })
  );
  handleRedirectCallback$ = this.auth0Client$.pipe(
    concatMap((client: Auth0Client) =>
      from(client.handleRedirectCallback(this.currentHref))
    )
  );
  // Create subject and public observable of user profile data
  private userProfileSubject$ = new BehaviorSubject<any>(undefined);
  userProfile$ = this.userProfileSubject$.asObservable();
  // Create a local property for login status
  loggedIn = false;

  constructor(
    private router: Router,
    private storageService: StorageService
  ) {

    // On initial load, check authentication state with authorization server
    // Set up local auth streams if user is already authenticated
    const params = this.currentHref;
    if (params.includes('code=') && params.includes('state=')) {
      this.handleAuthCallback();
    } else {
      this.localAuthSetup();
    }
    // this.handlerReturnToAferlogout();
  }

  handlerReturnToAferlogout() {
    const { query } = querystring.parseUrl(this.currentHref);
    const returnTo = query.returnTo;
    if (returnTo) {
      const target = this.getTargetRouteFromReturnTo(returnTo);
      this.router.navigate([target]);
    }
  }

  // When calling, options can be passed if desired
  // https://auth0.github.io/auth0-spa-js/classes/auth0client.html#getuser
  getUser$(options?): Observable<any> {
    return this.auth0Client$.pipe(
      concatMap((client: Auth0Client) => from(client.getUser(options))),
      tap((user) => {
        this.userProfileSubject$.next(user);
      })
    );
  }

  private localAuthSetup() {
    // This should only be called on app initialization
    // Set up local authentication streams
    const checkAuth$ = this.isAuthenticated$.pipe(
      concatMap((loggedIn: boolean) => {
        if (loggedIn) {
          // If authenticated, get user and set in app
          // NOTE: you could pass options here if needed
          return this.getUser$();
        }
        this.auth0Client$
          .pipe(concatMap((client: Auth0Client) => from(client.checkSession())))
          .subscribe((data) => { });
          console.log('User not logged in');
          this.userProfileSubject$.next(null);
        // If not authenticated, return stream that emits 'false'
        return of(loggedIn);
      })
    );
    checkAuth$.subscribe();
  }

  login(redirectPath: string = '/') {
    // A desired redirect path can be passed to login method
    // (e.g., from a route guard)
    // Ensure Auth0 client instance exists
    this.auth0Client$.subscribe((client: Auth0Client) => {
      // Call method to log in
      const type = JSON.parse(this.storageService.getItem('type'));
      const redirectConsole = (type === 'Projects') ? AppSettings.PROJECT_CONSOLE_LINK_V2 : AppSettings.CORPORATE_CONSOLE_LINK_V2;
      client.loginWithRedirect({
        redirect_uri: EnvConfig.default[redirectConsole],
        appState: { target: redirectPath },
      });
    });
  }

  private getTargetRouteFromAppState(appState) {
    if (!appState) {
      return '/';
    }

    const { returnTo, target, targetUrl } = appState;

    return (
      this.getTargetRouteFromReturnTo(returnTo) || target || targetUrl || '/'
    );
  }

  private getTargetRouteFromReturnTo(returnTo) {
    if (!returnTo) {
      return '';
    }

    const { fragmentIdentifier } = querystring.parseUrl(returnTo, {
      parseFragmentIdentifier: true,
    });

    if (fragmentIdentifier) {
      return fragmentIdentifier;
    }

    const { pathname } = new Url(returnTo);
    return pathname || '/';
  }

  private handleAuthCallback() {
    // Call when app reloads after user logs in with Auth0
    const params = this.currentHref;

    if (params.includes('code=') && params.includes('state=')) {
      let targetRoute: string; // Path to redirect to after login processsed
      const authComplete$ = this.handleRedirectCallback$.pipe(
        // Have client, now call method to handle auth callback redirect
        tap((cbRes: any) => {
          targetRoute = this.getTargetRouteFromAppState(cbRes.appState);
        }),
        concatMap(() => {
          // Redirect callback complete; get user and login status
          return combineLatest([this.getUser$(), this.isAuthenticated$]);
        })
      );
      // Subscribe to authentication completion observable
      // Response will be an array of user and login status
      authComplete$.subscribe(() => {
        // console.log('navigating too', {
        //   current: this.currentHref,
        //   targetRoute,
        //   href: window.location.href,
        // });
        // Redirect to target route after callback processing
        // *info: this url change will remove the code and state from the URL
        // * this is need to avoid invalid state in the next refresh
        // this.router.navigate([targetRoute]);
        if (targetRoute !== '/') {
          const type = JSON.parse(this.storageService.getItem('type'));
          const redirectConsole = (type === 'Projects') ? AppSettings.PROJECT_CONSOLE_LINK_V2 : AppSettings.CORPORATE_CONSOLE_LINK_V2;
          window.open(EnvConfig.default[redirectConsole] + REDIRECT_AUTH_ROUTE, '_self');
        } else {
          this.router.navigate([targetRoute]);
        }
      });
    }
  }

  logout() {
    const { query, fragmentIdentifier } = querystring.parseUrl(
      window.location.href,
      { parseFragmentIdentifier: true }
    );

    const qs = {
      ...query,
      returnTo: window.location.href,
    };

    const searchStr = querystring.stringify(qs);
    const searchPart = searchStr ? `?${searchStr}` : '';

    const fragmentPart = fragmentIdentifier ? `#${fragmentIdentifier}` : '';

    const request = {
      client_id: this.auth0Options.clientId,
      returnTo: `${window.location.origin}${searchPart}${fragmentPart}`,
    };

    this.auth0Client$.subscribe((client: Auth0Client) =>
      client.logout(request)
    );
  }

  getTokenSilently$(options?): Observable<any> {
    return this.auth0Client$.pipe(
      concatMap((client: Auth0Client) => from(client.getTokenSilently(options)))
    );
  }

  getIdToken$(options?): Observable<any> {
    return this.auth0Client$.pipe(
      // *info: if getIdToken fails , just return empty in the catchError
      concatMap((client: Auth0Client) =>
        from(client.getIdTokenClaims(options))
      ),
      concatMap((claims: any) => of((claims && claims.__raw) || '')),
      catchError(() => of(''))
    );
  }
}
