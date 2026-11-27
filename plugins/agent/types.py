from pydantic import BaseModel, ConfigDict, Field


class Bulb(BaseModel):
    """A lamp fitting from a photo or a product screenshot."""

    model_config = ConfigDict(extra="forbid")

    base: str | None = Field(default=None, description="Cap, such as E27 or GU10.")
    shape: str | None = Field(
        default=None, description="Envelope, such as A60 or candle."
    )


class ReferenceShot(BaseModel):
    """A photograph of a real object, not app chrome."""

    model_config = ConfigDict(extra="forbid")

    subject: str | None = Field(default=None, description="What the photo is of.")


class Link(BaseModel):
    """A web page or post you saved as a URL."""

    model_config = ConfigDict(extra="forbid")

    url: str | None = Field(default=None, description="URL you saved.")
    title: str | None = Field(
        default=None, description="Title you typed when you saved it."
    )
