class CustomerNotFoundError(LookupError):
    pass


class ReservationNotFoundError(LookupError):
    pass


class SegmentNotFoundError(LookupError):
    pass


class FlightNotFoundError(LookupError):
    pass
