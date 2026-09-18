class CustomerNotFoundError(LookupError):
    pass


class ReservationNotFoundError(LookupError):
    pass


class SegmentNotFoundError(LookupError):
    pass


class FlightNotFoundError(LookupError):
    pass


class TravelCreditNotFoundError(LookupError):
    pass


class RebookingVerificationError(RuntimeError):
    pass


class RebookingRuleError(ValueError):
    pass
