from django import forms

from trove.services.setup_code import code_matches


class SetupCodeForm(forms.Form):
    code = forms.CharField(label="Setup code", max_length=32)

    def clean_code(self):
        code = self.cleaned_data["code"]
        if not code_matches(code):
            raise forms.ValidationError("That setup code is wrong or already used.")
        return code
