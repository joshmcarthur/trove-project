from allauth.mfa.webauthn.forms import SignupWebAuthnForm
from allauth.mfa.webauthn.internal import auth, flows
from django.contrib.auth import login
from django.http import HttpResponse, HttpResponseRedirect
from django.shortcuts import render
from django.urls import reverse
from django.views import View

from trove.forms.setup_code import SetupCodeForm
from trove.services.owner import ensure_owner, owner_has_passkey
from trove.services.setup_code import consume_setup_code

SESSION_KEY = "trove_setup_ok"


class SetupView(View):
    def get(self, request):
        if owner_has_passkey():
            return HttpResponse("The owner passkey is already registered.", status=409)
        if request.session.get(SESSION_KEY):
            return self._passkey_page(request)
        return render(request, "trove/setup_code.html", {"form": SetupCodeForm()})

    def post(self, request):
        if owner_has_passkey():
            return HttpResponse("The owner passkey is already registered.", status=409)
        if request.session.get(SESSION_KEY) and "credential" in request.POST:
            return self._finish_passkey(request)
        form = SetupCodeForm(request.POST)
        if not form.is_valid():
            return render(request, "trove/setup_code.html", {"form": form}, status=400)
        request.session[SESSION_KEY] = True
        ensure_owner()
        return self._passkey_page(request)

    def _passkey_page(self, request, form=None):
        user = ensure_owner()
        creation_options = auth.begin_registration(user, passwordless=True)
        if form is None:
            form = SignupWebAuthnForm(user=user)
        return render(
            request,
            "trove/setup_passkey.html",
            {"form": form, "js_data": {"creation_options": creation_options}},
        )

    def _finish_passkey(self, request):
        user = ensure_owner()
        form = SignupWebAuthnForm(request.POST, user=user)
        if not form.is_valid():
            return self._passkey_page(request, form=form)
        flows.signup_authenticator(
            request,
            user=user,
            name=form.cleaned_data["name"],
            credential=form.cleaned_data["credential"],
        )
        consume_setup_code()
        request.session.pop(SESSION_KEY, None)
        login(request, user, backend="django.contrib.auth.backends.ModelBackend")
        return HttpResponseRedirect(reverse("admin:index"))
