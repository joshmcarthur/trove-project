"""Django model factories.

Kernel (#126) adds DjangoModelFactory subclasses for Item, Blob, Claim, Link,
Activity, and ConfigEntry. Factories must honour the database router: facts
on `data`, ConfigEntry on `default`. Later issues add factories here rather
than a second module.
"""
