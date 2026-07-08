package handlers

import (
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ResetPasswordPage serves a mobile-friendly web page for password reset links from email.
func (h *AuthHandler) ResetPasswordPage(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(invalidResetLinkHTML()))
		return
	}

	escapedToken := html.EscapeString(token)
	page := strings.Replace(resetPasswordPageHTML, "{{TOKEN}}", escapedToken, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func invalidResetLinkHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>BillGenie — Invalid link</title>
  <style>
    body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; margin: 0; padding: 24px; }
    .card { max-width: 420px; margin: 40px auto; background: #1e293b; border-radius: 12px; padding: 24px; }
    h1 { font-size: 1.25rem; margin: 0 0 12px; color: #fff; }
    p { color: #94a3b8; line-height: 1.5; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Invalid reset link</h1>
    <p>This password reset link is missing or invalid. Request a new one from the BillGenie app.</p>
  </div>
</body>
</html>`
}

const resetPasswordPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>BillGenie — Reset password</title>
  <style>
    * { box-sizing: border-box; }
    body {
      font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
      background: linear-gradient(160deg, #0f172a 0%, #1e293b 100%);
      color: #e2e8f0;
      margin: 0;
      min-height: 100vh;
      padding: 20px;
    }
    .card {
      max-width: 420px;
      margin: 24px auto;
      background: #1e293b;
      border: 1px solid #334155;
      border-radius: 16px;
      padding: 28px 24px;
      box-shadow: 0 12px 40px rgba(0,0,0,0.35);
    }
    h1 { font-size: 1.35rem; margin: 0 0 8px; color: #fff; }
    .sub { color: #94a3b8; font-size: 0.95rem; margin: 0 0 20px; line-height: 1.45; }
    label { display: block; font-size: 0.85rem; color: #cbd5e1; margin-bottom: 6px; }
    input {
      width: 100%;
      padding: 12px 14px;
      margin-bottom: 14px;
      border: 1px solid #475569;
      border-radius: 10px;
      background: #0f172a;
      color: #fff;
      font-size: 16px;
    }
    input:focus { outline: 2px solid #10b981; border-color: #10b981; }
    button {
      width: 100%;
      padding: 14px;
      border: none;
      border-radius: 10px;
      background: #059669;
      color: #fff;
      font-size: 1rem;
      font-weight: 600;
      cursor: pointer;
      margin-top: 4px;
    }
    button:disabled { opacity: 0.55; cursor: not-allowed; }
    .app-link {
      display: block;
      text-align: center;
      margin-top: 16px;
      color: #34d399;
      text-decoration: none;
      font-size: 0.9rem;
    }
    .msg { margin-top: 16px; padding: 12px; border-radius: 8px; font-size: 0.9rem; display: none; }
    .msg.error { display: block; background: #450a0a; color: #fecaca; border: 1px solid #991b1b; }
    .msg.ok { display: block; background: #052e16; color: #bbf7d0; border: 1px solid #166534; }
    .hint { font-size: 0.8rem; color: #64748b; margin-top: 12px; text-align: center; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Reset your password</h1>
    <p class="sub">Choose a new password for your BillGenie admin account. This link expires in 1 hour.</p>

    <form id="reset-form">
      <label for="password">New password</label>
      <input id="password" type="password" name="password" minlength="6" autocomplete="new-password" required placeholder="At least 6 characters" />

      <label for="confirm">Confirm password</label>
      <input id="confirm" type="password" name="confirm" minlength="6" autocomplete="new-password" required placeholder="Repeat password" />

      <button type="submit" id="submit-btn">Update password</button>
    </form>

    <a class="app-link" id="open-app" href="#">Open in BillGenie app</a>
    <p class="hint">On your phone, you can reset here in the browser or open the app.</p>
    <div id="message" class="msg"></div>
  </div>

  <script>
    (function () {
      var token = "{{TOKEN}}";
      var deepLink = "billgenie://reset-password?token=" + encodeURIComponent(token);
      var openApp = document.getElementById("open-app");
      openApp.href = deepLink;

      var isMobile = /Android|iPhone|iPad|iPod/i.test(navigator.userAgent || "");
      if (isMobile) {
        window.location.replace(deepLink);
      }

      var form = document.getElementById("reset-form");
      var msg = document.getElementById("message");
      var btn = document.getElementById("submit-btn");

      function showMsg(text, ok) {
        msg.textContent = text;
        msg.className = "msg " + (ok ? "ok" : "error");
      }

      form.addEventListener("submit", function (e) {
        e.preventDefault();
        var password = document.getElementById("password").value;
        var confirm = document.getElementById("confirm").value;
        if (password.length < 6) {
          showMsg("Password must be at least 6 characters.", false);
          return;
        }
        if (password !== confirm) {
          showMsg("Passwords do not match.", false);
          return;
        }
        btn.disabled = true;
        btn.textContent = "Updating…";
        fetch("/auth/reset-password", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token: token, new_password: password })
        })
          .then(function (res) { return res.json().then(function (data) { return { ok: res.ok, data: data }; }); })
          .then(function (result) {
            if (result.ok) {
              showMsg(result.data.message || "Password updated. You can now sign in to BillGenie.", true);
              form.style.display = "none";
              openApp.textContent = "Open BillGenie app to sign in";
            } else {
              showMsg(result.data.error || "Reset failed. The link may have expired.", false);
              btn.disabled = false;
              btn.textContent = "Update password";
            }
          })
          .catch(function () {
            showMsg("Network error. Please try again.", false);
            btn.disabled = false;
            btn.textContent = "Update password";
          });
      });
    })();
  </script>
</body>
</html>`
