#pragma once

#include <google/protobuf/message.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <rpc/clnt.h>
#include <rpc/proto/rpc.pb.h>
#include <serr/serr.h>
#include <util/log/log.h>

#include <atomic>
#include <expected>

namespace sigmaos {
namespace rpc {

// Checks if a protobuf has a field named "blob"
bool has_blob_field(const google::protobuf::Message &msg);

// Extract a blob from a protobuf, if one exists. Returns the blob and a boolean
// indicating whether a blob was found.
// If the given RPC has a blob field, extract its IOVecs.
void extract_blob_iov(google::protobuf::Message &msg,
                      std::shared_ptr<sigmaos::io::iovec::IOVec> dst);

// If the given RPC has a blob field, extract its IOVecs.
void set_blob_iov(std::shared_ptr<sigmaos::io::iovec::IOVec> src,
                  google::protobuf::Message &msg);

};  // namespace rpc
};  // namespace sigmaos
