package io.github.stwdev.powergame;

import android.app.Activity;
import android.graphics.Color;
import android.net.Uri;
import android.os.Bundle;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.HashMap;
import java.util.Map;

/**
 * Shows the game, which is bundled in assets/www, in a WebView.
 *
 * Files are served from a private https origin by intercepting requests, so
 * the page gets a proper secure context (Web Workers, WebAssembly) without
 * enabling file:// access. Every request for any other origin is refused,
 * which means the app never touches the network.
 */
public class MainActivity extends Activity {
    private static final String HOST = "appassets.androidplatform.net";
    private static final String CSP = "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; "
            + "worker-src 'self'; connect-src 'self'; style-src 'self'; img-src 'self' data:; "
            + "manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'";

    private WebView web;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        getWindow().setStatusBarColor(Color.rgb(0x14, 0x18, 0x1f));
        web = new WebView(this);
        web.setBackgroundColor(Color.rgb(0x14, 0x18, 0x1f));
        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true); // saved games live in localStorage
        s.setAllowFileAccess(false);
        s.setAllowContentAccess(false);
        s.setGeolocationEnabled(false);
        s.setSupportMultipleWindows(false);
        s.setJavaScriptCanOpenWindowsAutomatically(false);
        s.setUserAgentString(s.getUserAgentString() + " PowerGameApp");
        web.setWebViewClient(new WebViewClient() {
            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest req) {
                return serve(req.getUrl());
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest req) {
                return !isOurs(req.getUrl()); // never navigate away from the game
            }
        });
        if (state != null) {
            web.restoreState(state);
        } else {
            web.loadUrl("https://" + HOST + "/index.html");
        }
        setContentView(web);
    }

    @Override
    protected void onSaveInstanceState(Bundle out) {
        super.onSaveInstanceState(out);
        web.saveState(out);
    }

    @Override
    protected void onDestroy() {
        web.destroy();
        super.onDestroy();
    }

    private static boolean isOurs(Uri u) {
        return "https".equals(u.getScheme()) && HOST.equals(u.getHost()) && u.getPort() == -1;
    }

    private WebResourceResponse serve(Uri u) {
        if (!isOurs(u)) {
            return error(403, "Forbidden");
        }
        String path = u.getPath();
        if (path == null || path.equals("/") || path.isEmpty()) {
            path = "/index.html";
        }
        path = path.substring(1);
        for (String part : path.split("/")) {
            if (part.isEmpty() || part.startsWith(".")) {
                return error(404, "Not Found");
            }
        }
        InputStream in;
        try {
            in = getAssets().open("www/" + path);
        } catch (IOException e) {
            return error(404, "Not Found");
        }
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Security-Policy", CSP);
        headers.put("X-Content-Type-Options", "nosniff");
        headers.put("Cache-Control", "no-store");
        String type = mime(path);
        String charset = type.startsWith("text/") || type.endsWith("json") ? "utf-8" : null;
        return new WebResourceResponse(type, charset, 200, "OK", headers, in);
    }

    private static WebResourceResponse error(int code, String reason) {
        return new WebResourceResponse("text/plain", "utf-8", code, reason,
                new HashMap<String, String>(), new ByteArrayInputStream(new byte[0]));
    }

    private static String mime(String path) {
        if (path.endsWith(".html")) return "text/html";
        if (path.endsWith(".js")) return "text/javascript";
        if (path.endsWith(".css")) return "text/css";
        if (path.endsWith(".wasm")) return "application/wasm";
        if (path.endsWith(".webmanifest")) return "application/manifest+json";
        if (path.endsWith(".png")) return "image/png";
        if (path.endsWith(".svg")) return "image/svg+xml";
        return "application/octet-stream";
    }
}
