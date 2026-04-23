# heap_abi.packet.md
status,draft
scope,project:fibtransponder/internal/deltaqueue
kind,heap_abi
non_goal,semantic_identity

[core]
rule,agents may inspect summaries
rule,agents may propose bounded heap ops
rule,only runtime may mutate structural internals
rule,heap objects are surfaced runtime objects not semantic identities

[types.handle]
name,HeapHandle
field,handle_id:string
field,heap_id:string
field,semantic_ref:string
field,key:int64
field,generation:uint32

[types.summary]
name,HeapSummary
field,heap_id:string
field,min_handle:string
field,node_count:uint32
field,root_count:uint32
field,dirty:bool
field,top_k:list[string]
field,last_op:string?
field,last_mutation_tick:uint64?

[ops.read_only]
op,PEEK_MIN
op,HANDLE_EXISTS
op,GET_KEY
op,GET_SUMMARY

[ops.mutating]
op,INSERT
op,DECREASE_KEY
op,EXTRACT_MIN
op,MELD
op,DELETE_HANDLE

[forbidden]
item,raw_pointer_rewrites
item,child_list_rewrites
item,sibling_list_rewrites
item,standalone_mark_bit_edits
item,raw_consolidation_steps
item,explicit_cut_cascade_sequences
item,full_heap_dump_by_default

[wire]
shape,"scope,type,target,input[,guard]"

[wire.example.1]
scope,project:fibtransponder
type,heap_op
target,frontier_heap
input,"DECREASE_KEY handle=h_482 old=41 new=12"
guard,"preserve_heap_order|preserve_handle_validity"

[packet]
field,heap_id
field,op
field,handle_id
field,semantic_ref
field,old_key?
field,new_key?
field,preserve:list[string]
field,proof_target:list[string]

[packet.example]
heap_id,frontier_heap
op,DECREASE_KEY
handle_id,h_482
semantic_ref,mem_482
old_key,41
new_key,12
preserve,heap_order|handle_validity|runtime_only_mutation
proof_target,handle_reachable|min_pointer_valid

[contract.INSERT.pre]
check,handle_not_present
check,key_present

[contract.INSERT.post]
check,handle_present
check,heap_order_holds
check,min_pointer_valid

[contract.DECREASE_KEY.pre]
check,handle_exists
check,new_key_lt_old_key

[contract.DECREASE_KEY.post]
check,key_updated
check,heap_order_holds
check,handle_reachable
check,min_pointer_valid
note,runtime_may_cut_or_cascade_internally

[contract.EXTRACT_MIN.pre]
check,heap_non_empty

[contract.EXTRACT_MIN.post]
check,previous_min_removed
check,returned_handle_named
check,heap_order_holds
check,min_pointer_valid
note,runtime_may_consolidate_internally

[contract.MELD.pre]
check,heap_a_exists
check,heap_b_exists
check,meld_policy_allows

[contract.MELD.post]
check,all_handles_reachable_in_target
check,source_heap_state_named_by_runtime
check,min_pointer_valid

[contract.DELETE_HANDLE.pre]
check,handle_exists

[contract.DELETE_HANDLE.post]
check,handle_removed_from_active_heap
check,heap_order_holds
check,min_pointer_valid

[invariant.1]
name,RuntimeOwnsStructure
rule,only_runtime_mutates_structural_heap_internals

[invariant.2]
name,HandleStability
rule,all_ops_address_handles_by_handle_id_not_pointer_or_position

[invariant.3]
name,SurfaceOnly
rule,heap_handles_refer_to_surfaced_runtime_objects_only

[invariant.4]
name,ExplicitLaziness
rule,deferred_work_must_be_visible_in_summary_via_dirty_or_equivalent_flag

[invariant.5]
name,NoRawPointerTalk
rule,cross_agent_packets_must_not_depend_on_raw_structural_addresses

[proof_shape]
field,pre:list[string]
field,effect:list[string]
field,post:list[string]

[proof.example]
pre,handle_exists(h_482)|new_key_lt_old_key
effect,key(h_482)=12
post,heap_order_holds|h_482_reachable|min_pointer_valid

[summary_surface]
field,heap_id
field,min_handle
field,node_count
field,root_count
field,dirty
field,top_k

[relationship]
semantic_layer,"may request surface reprioritization for memory identity M"
heap_layer,"translates that into bounded heap op on surfaced handle linked to M"
rule,heap_does_not_define_semantic_identity

[non_goals]
item,fibonacci_memory_identity
item,rewrite_equivalence
item,promote_semantics_beyond_surface_reprioritization
item,semantic_tombstone_truth
item,classifier_logic
