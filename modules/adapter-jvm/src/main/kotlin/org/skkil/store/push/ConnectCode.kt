package org.skkil.store.push

/** Connect RPC error codes with their wire names and HTTP statuses. */
public enum class ConnectCode(
    /** The code as written in a Connect error body. */
    public val wireName: String,
    /** The HTTP status Connect maps the code to. */
    public val httpStatus: Int,
) {
    CANCELED("canceled", 499),
    UNKNOWN("unknown", 500),
    INVALID_ARGUMENT("invalid_argument", 400),
    DEADLINE_EXCEEDED("deadline_exceeded", 504),
    NOT_FOUND("not_found", 404),
    ALREADY_EXISTS("already_exists", 409),
    PERMISSION_DENIED("permission_denied", 403),
    RESOURCE_EXHAUSTED("resource_exhausted", 429),
    FAILED_PRECONDITION("failed_precondition", 400),
    ABORTED("aborted", 409),
    OUT_OF_RANGE("out_of_range", 400),
    UNIMPLEMENTED("unimplemented", 501),
    INTERNAL("internal", 500),
    UNAVAILABLE("unavailable", 503),
    DATA_LOSS("data_loss", 500),
    UNAUTHENTICATED("unauthenticated", 401),
}
