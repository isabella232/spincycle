/*
  This data is used by tests in the request-manager/jc package.
*/

-- an invalid job chain
INSERT INTO request_archives (request_id, create_request, args, job_chain) VALUES ("cd724fd12092", '{"some":"param"}', '', '{"requestId":"8bff5def4f3fvh78skjy","jobs":{"this is not valid json"}},"adjacencyList":null,"state":4}');
